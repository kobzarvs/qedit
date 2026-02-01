package editor

import (
	"errors"
	"os"
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

func snapshotFromInfo(info os.FileInfo) fileSnapshot {
	return fileSnapshot{
		modTime: info.ModTime(),
		size:    info.Size(),
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
	if e.filename == "" {
		e.fileSnapshot = fileSnapshot{}
		return nil
	}
	info, err := os.Stat(e.filename)
	if err != nil {
		if os.IsNotExist(err) {
			e.fileSnapshot = fileSnapshot{exists: false, valid: true}
			return nil
		}
		return err
	}
	e.fileSnapshot = snapshotFromInfo(info)
	return nil
}

// CheckExternalChange compares the last known file snapshot with the current file state.
func (e *Editor) CheckExternalChange() (ExternalChange, error) {
	if e.filename == "" || !e.fileSnapshot.valid {
		return ExternalChangeNone, nil
	}
	info, err := os.Stat(e.filename)
	if err != nil {
		if os.IsNotExist(err) {
			if e.fileSnapshot.exists {
				return ExternalChangeDeleted, nil
			}
			return ExternalChangeNone, nil
		}
		return ExternalChangeNone, err
	}
	current := snapshotFromInfo(info)
	if e.fileSnapshot.equal(current) {
		return ExternalChangeNone, nil
	}
	return ExternalChangeModified, nil
}

// ReloadFromDisk replaces the buffer with on-disk contents.
func (e *Editor) ReloadFromDisk(force bool) error {
	if e.filename == "" {
		return errors.New("no file name")
	}
	if e.dirty && !force {
		return errors.New("unsaved changes (use :e!)")
	}
	data, err := os.ReadFile(e.filename)
	if err != nil {
		return err
	}
	e.replaceBuffer(string(data), false)
	e.selectionActive = false
	e.externalChange = ExternalChangeNone
	_ = e.syncFileSnapshot()
	_ = e.LoadUndoHistory()
	return nil
}

// ExternalChange reports the current pending external-change state.
func (e *Editor) ExternalChange() ExternalChange {
	return e.externalChange
}

// SetExternalChange marks an external change as pending.
func (e *Editor) SetExternalChange(change ExternalChange) {
	e.externalChange = change
}

// ClearExternalChange clears any pending external-change state.
func (e *Editor) ClearExternalChange() {
	e.externalChange = ExternalChangeNone
}

// IsDirty reports whether the buffer has unsaved changes.
func (e *Editor) IsDirty() bool {
	return e.dirty
}

// Filename returns the current buffer filename.
func (e *Editor) Filename() string {
	return e.filename
}
