package editor

import (
	"errors"
	"time"
)

// ExternalChange describes a detected on-disk change for the current file.
type ExternalChange int

const (
	ExternalChangeNone ExternalChange = iota
	ExternalChangeModified
	ExternalChangeDeleted
)

type fileSnapshot struct {
	modTime time.Time
	size    int64
	exists  bool
	valid   bool
}

func snapshotFromMetadata(meta FileMetadata) fileSnapshot {
	return fileSnapshot{
		modTime: meta.ModTime,
		size:    meta.Size,
		exists:  true,
		valid:   true,
	}
}

func (s fileSnapshot) equal(other fileSnapshot) bool {
	if s.valid != other.valid {
		return false
	}
	if !s.valid {
		return true
	}
	if s.exists != other.exists {
		return false
	}
	if !s.exists {
		return true
	}
	return s.size == other.size && s.modTime.Equal(other.modTime)
}

func (e *Editor) syncFileSnapshot() error {
	if e.document.filename == "" {
		e.file.snapshot = fileSnapshot{}
		return nil
	}
	if e.runtime.workspace == nil || !e.runtime.workspace.HasFileStore() {
		return errFileStoreUnavailable()
	}
	info, err := e.runtime.workspace.Stat(e.document.filename)
	if err != nil {
		if e.runtime.workspace.IsNotExist(err) {
			e.file.snapshot = fileSnapshot{exists: false, valid: true}
			return nil
		}
		return err
	}
	e.file.snapshot = snapshotFromMetadata(info)
	return nil
}

// CheckExternalChange compares the last known file snapshot with the current file state.
func (e *Editor) CheckExternalChange() (ExternalChange, error) {
	if e.document.filename == "" || !e.file.snapshot.valid {
		return ExternalChangeNone, nil
	}
	if e.runtime.workspace == nil || !e.runtime.workspace.HasFileStore() {
		return ExternalChangeNone, errFileStoreUnavailable()
	}
	info, err := e.runtime.workspace.Stat(e.document.filename)
	if err != nil {
		if e.runtime.workspace.IsNotExist(err) {
			if e.file.snapshot.exists {
				return ExternalChangeDeleted, nil
			}
			return ExternalChangeNone, nil
		}
		return ExternalChangeNone, err
	}
	current := snapshotFromMetadata(info)
	if e.file.snapshot.equal(current) {
		return ExternalChangeNone, nil
	}
	return ExternalChangeModified, nil
}

// ReloadFromDisk replaces the buffer with on-disk contents.
func (e *Editor) ReloadFromDisk(force bool) error {
	path, err := e.prepareReload(force)
	if err != nil {
		return err
	}
	if e.runtime.workspace == nil || !e.runtime.workspace.HasFileStore() {
		return errFileStoreUnavailable()
	}
	data, err := e.runtime.workspace.Read(path)
	if err != nil {
		return err
	}
	e.ApplyReloadedContent(data)
	return nil
}

// ExternalChange reports the current pending external-change state.
func (e *Editor) ExternalChange() ExternalChange {
	return e.file.externalChange
}

// SetExternalChange marks an external change as pending.
func (e *Editor) SetExternalChange(change ExternalChange) {
	e.file.externalChange = change
	e.updateDirty()
}

// ClearExternalChange clears any pending external-change state.
func (e *Editor) ClearExternalChange() {
	e.file.externalChange = ExternalChangeNone
	e.updateDirty()
}

// AutoReloadInProgress reports whether an auto-reload is running.
func (e *Editor) AutoReloadInProgress() bool {
	return e.file.autoReloadInProgress
}

// SetAutoReloadInProgress updates auto-reload progress state.
func (e *Editor) SetAutoReloadInProgress(inProgress bool) {
	e.file.autoReloadInProgress = inProgress
	if inProgress {
		e.ui.statusMessage = "auto reload... waiting for file write to finish"
		return
	}
	if e.ui.statusMessage == "auto reload... waiting for file write to finish" {
		e.ui.statusMessage = ""
	}
}

// AutoReloadOnChanges reports whether auto-reload on external changes is enabled.
func (e *Editor) AutoReloadOnChanges() bool {
	return e.file.autoReloadOnChanges
}

// SetAutoReloadOnChanges updates the runtime auto-reload setting.
func (e *Editor) SetAutoReloadOnChanges(enabled bool) {
	e.file.autoReloadOnChanges = enabled
}

// SetSidebarWidthConfig updates the runtime sidebar width setting.
func (e *Editor) SetSidebarWidthConfig(width string) {
	if e.sidebar == nil || width == "" {
		return
	}
	e.sidebar.WidthConfig = width
}

// IsDirty reports whether the buffer has unsaved changes.
func (e *Editor) IsDirty() bool {
	return e.document.dirty
}

// HasLocalChanges reports whether the buffer has local unsaved edits.
func (e *Editor) HasLocalChanges() bool {
	return len(e.undo) != e.savePoint
}

// MarkExternalDirty marks the buffer dirty due to on-disk changes.
func (e *Editor) MarkExternalDirty() {
	e.updateDirty()
}

// MergeExternalContent merges on-disk content into the current buffer.
// Returns true if conflicts were produced.
func (e *Editor) MergeExternalContent(remote string) (bool, error) {
	remoteNormalized := string(normalizeNewlines([]byte(remote)))
	base := e.file.diskContent
	if base == "" {
		base = e.Content()
	}
	local := e.Content()
	if local == remoteNormalized {
		e.file.diskContent = remoteNormalized
		e.file.externalChange = ExternalChangeNone
		e.updateDirty()
		_ = e.syncFileSnapshot()
		return false, nil
	}
	if local == base {
		e.file.externalChange = ExternalChangeNone
		e.replaceBuffer(remoteNormalized, false)
		e.selectionActive = false
		e.file.diskContent = remoteNormalized
		e.updateDirty()
		e.resetConflictBlocks()
		_ = e.syncFileSnapshot()
		return false, nil
	}
	if e.runtime.workspace == nil || !e.runtime.workspace.HasMerger() {
		return false, errMergerUnavailable()
	}
	merged, conflict, err := e.runtime.workspace.Merge(base, local, remoteNormalized)
	if err != nil {
		return false, err
	}
	e.file.externalChange = ExternalChangeNone
	if conflict {
		cleaned, blocks := buildConflictView(merged)
		e.replaceBuffer(cleaned, true)
		e.conflicts.blocks = blocks
		e.conflicts.dirty = false
		if e.mode == ModeNormal {
			e.mode = ModeMerge
		}
	} else {
		e.replaceBuffer(merged, true)
		e.resetConflictBlocks()
		if e.mode == ModeMerge {
			e.mode = ModeNormal
		}
	}
	e.selectionActive = false
	e.file.diskContent = remoteNormalized
	e.updateDirty()
	_ = e.syncFileSnapshot()
	return conflict, nil
}

// Filename returns the current buffer filename.
func (e *Editor) Filename() string {
	return e.document.filename
}

func errFileStoreUnavailable() error {
	return errors.New("file store unavailable")
}

func errMergerUnavailable() error {
	return errors.New("merger unavailable")
}
