package app

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/logger"
)

type autoReloadResult struct {
	seq  uint64
	data []byte
	err  error
}

type externalFileMonitor struct {
	screen                   tcell.Screen
	ed                       *editor.Editor
	autoReloadMaxBytes       int64
	autoReloadRetries        int
	autoReloadStabilizeDelay time.Duration

	fileWatcher       *fsnotify.Watcher
	fileEvents        chan time.Time
	pendingFileChange bool
	lastFileEvent     time.Time
	lastFileCheck     time.Time
	watchedPath       string

	autoReloadSeq     uint64
	autoReloadActive  bool
	autoReloadPending bool
	autoReloadResults chan autoReloadResult
	fileStore         editor.FileStore
}

func newExternalFileMonitor(screen tcell.Screen, ed *editor.Editor, fileStore editor.FileStore, autoReloadMaxBytes int64, autoReloadRetries int, autoReloadStabilizeDelay time.Duration) *externalFileMonitor {
	return &externalFileMonitor{
		screen:                   screen,
		ed:                       ed,
		fileStore:                fileStore,
		autoReloadMaxBytes:       autoReloadMaxBytes,
		autoReloadRetries:        autoReloadRetries,
		autoReloadStabilizeDelay: autoReloadStabilizeDelay,
		autoReloadResults:        make(chan autoReloadResult, 1),
	}
}

func (m *externalFileMonitor) Close() {
	if m.fileWatcher != nil {
		_ = m.fileWatcher.Close()
		m.fileWatcher = nil
	}
}

func (m *externalFileMonitor) Watch(path string) {
	if m.fileWatcher != nil {
		_ = m.fileWatcher.Close()
		m.fileWatcher = nil
	}
	m.fileEvents = nil
	m.watchedPath = ""
	m.pendingFileChange = false
	m.lastFileEvent = time.Time{}

	if path == "" {
		return
	}
	absPath := normalizeAppPath(m.fileStore, path)
	m.watchedPath = absPath
	watchedDir := filepath.Dir(absPath)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Warn("file watcher unavailable", "error", err)
		return
	}
	if err := watcher.Add(watchedDir); err != nil {
		_ = watcher.Close()
		logger.Warn("file watcher disabled", "error", err)
		return
	}
	m.fileWatcher = watcher
	m.fileEvents = make(chan time.Time, 1)
	go m.watchLoop(watcher, m.fileEvents)
}

func (m *externalFileMonitor) watchLoop(w *fsnotify.Watcher, events chan time.Time) {
	sendEvent := func(ts time.Time) {
		select {
		case events <- ts:
			return
		default:
		}
		select {
		case <-events:
		default:
		}
		select {
		case events <- ts:
		default:
		}
	}

	for {
		select {
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 {
				continue
			}
			if filepath.Clean(ev.Name) != m.watchedPath {
				continue
			}
			sendEvent(time.Now())
			_ = m.screen.PostEvent(tcell.NewEventInterrupt(nil))
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			logger.Warn("file watcher error", "error", err)
		}
	}
}

func (m *externalFileMonitor) readFileStable(path string) ([]byte, error) {
	var lastErr error
	for i := 0; i < m.autoReloadRetries; i++ {
		infoBefore, err := m.fileStore.Stat(path)
		if err != nil {
			return nil, err
		}
		time.Sleep(m.autoReloadStabilizeDelay)
		infoMid, err := m.fileStore.Stat(path)
		if err != nil {
			return nil, err
		}
		if !infoBefore.ModTime.Equal(infoMid.ModTime) || infoBefore.Size != infoMid.Size {
			lastErr = fmt.Errorf("file changed during read")
			continue
		}
		data, err := m.fileStore.Read(path)
		if err != nil {
			lastErr = err
			time.Sleep(m.autoReloadStabilizeDelay)
			continue
		}
		infoAfter, err := m.fileStore.Stat(path)
		if err != nil {
			return nil, err
		}
		if infoMid.ModTime.Equal(infoAfter.ModTime) && infoMid.Size == infoAfter.Size && int64(len(data)) == infoAfter.Size {
			return data, nil
		}
		lastErr = fmt.Errorf("file changed during read")
		time.Sleep(m.autoReloadStabilizeDelay)
	}
	return nil, lastErr
}

func (m *externalFileMonitor) startAutoReload() {
	if m.watchedPath == "" {
		return
	}
	if m.autoReloadActive {
		m.autoReloadPending = true
		return
	}
	m.autoReloadActive = true
	m.autoReloadPending = false
	m.autoReloadSeq++
	seq := m.autoReloadSeq
	m.ed.SetAutoReloadInProgress(true)
	go func(path string, currentSeq uint64) {
		data, err := m.readFileStable(path)
		select {
		case m.autoReloadResults <- autoReloadResult{seq: currentSeq, data: data, err: err}:
		default:
			select {
			case <-m.autoReloadResults:
			default:
			}
			m.autoReloadResults <- autoReloadResult{seq: currentSeq, data: data, err: err}
		}
		_ = m.screen.PostEvent(tcell.NewEventInterrupt(nil))
	}(m.watchedPath, seq)
}

func (m *externalFileMonitor) applyExternalChange() {
	change, err := m.ed.CheckExternalChange()
	if err != nil {
		logger.Error("file change check failed", "error", err)
		return
	}
	if change == editor.ExternalChangeNone {
		if m.ed.ExternalChange() != editor.ExternalChangeNone {
			m.ed.ClearExternalChange()
		}
		return
	}
	if change == editor.ExternalChangeModified {
		if m.ed.AutoReloadOnChanges() {
			m.ed.ClearExternalChange()
			m.startAutoReload()
			return
		}
		largeFile := false
		if name := m.ed.Filename(); name != "" {
			if info, err := m.fileStore.Stat(name); err == nil && info.Size > m.autoReloadMaxBytes {
				largeFile = true
			}
		}
		if m.ed.ExternalChange() != change {
			m.ed.SetExternalChange(change)
			msg := "file changed on disk (use :e to reload)"
			if m.ed.HasLocalChanges() {
				msg = "file changed on disk (use :e! to reload)"
			}
			if largeFile {
				msg = "large file changed on disk (use :e to reload)"
				if m.ed.HasLocalChanges() {
					msg = "large file changed on disk (use :e! to reload)"
				}
			}
			m.ed.SetStatusMessage(msg)
		}
		m.ed.MarkExternalDirty()
		return
	}
	if m.ed.ExternalChange() != change {
		m.ed.SetExternalChange(change)
		switch change {
		case editor.ExternalChangeDeleted:
			m.ed.SetStatusMessage("file deleted on disk")
		default:
			m.ed.SetStatusMessage("file changed on disk (use :e! to reload)")
		}
	}
}

func (m *externalFileMonitor) ProcessWatcherEvents(now time.Time, debounce time.Duration) {
	if m.fileWatcher == nil {
		return
	}
	for {
		select {
		case ts := <-m.fileEvents:
			m.pendingFileChange = true
			if ts.After(m.lastFileEvent) {
				m.lastFileEvent = ts
			}
		default:
			goto done
		}
	}
done:
	if m.pendingFileChange && now.Sub(m.lastFileEvent) >= debounce {
		m.pendingFileChange = false
		m.applyExternalChange()
	}
}

func (m *externalFileMonitor) PollExternalChange(now time.Time, interval time.Duration) {
	if m.watchedPath == "" || now.Sub(m.lastFileCheck) <= interval {
		return
	}
	m.lastFileCheck = now
	m.applyExternalChange()
}

func (m *externalFileMonitor) HandleAutoReloadResults() {
	if !m.autoReloadActive {
		return
	}
	select {
	case res := <-m.autoReloadResults:
		if res.seq == m.autoReloadSeq {
			m.autoReloadActive = false
			m.ed.SetAutoReloadInProgress(false)
			if res.err != nil {
				m.ed.SetExternalChange(editor.ExternalChangeModified)
				m.ed.SetStatusMessage("auto reload failed: " + res.err.Error())
			} else if conflict, err := m.ed.MergeExternalContent(string(res.data)); err != nil {
				m.ed.SetExternalChange(editor.ExternalChangeModified)
				m.ed.SetStatusMessage("auto reload failed: " + err.Error())
			} else if conflict {
				m.ed.SetStatusMessage("auto reload merged with conflicts")
			} else {
				m.ed.SetStatusMessage("auto reload complete")
			}
		}
		if m.autoReloadPending && m.ed.AutoReloadOnChanges() {
			m.autoReloadPending = false
			m.startAutoReload()
		}
	default:
	}
}
