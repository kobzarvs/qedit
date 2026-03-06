package editor

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

var errTestUndoNotExist = errors.New("undo log not found")

type testUndoStore struct {
	loadData    map[string][]byte
	savedPath   string
	savedData   []byte
	removedPath string
}

func (s *testUndoStore) Load(path string) ([]byte, error) {
	data, ok := s.loadData[path]
	if !ok {
		return nil, errTestUndoNotExist
	}
	return append([]byte(nil), data...), nil
}

func (s *testUndoStore) Save(path string, data []byte) error {
	s.savedPath = path
	s.savedData = append([]byte(nil), data...)
	return nil
}

func (s *testUndoStore) Remove(path string) error {
	s.removedPath = path
	return nil
}

func (s *testUndoStore) IsNotExist(err error) bool {
	return errors.Is(err, errTestUndoNotExist)
}

func TestSaveUndoHistoryUsesRuntimeStore(t *testing.T) {
	e := newTestEditor("hello")
	store := &testUndoStore{}
	e.SetUndoStore(store)
	e.document.filename = "/tmp/file.txt"
	e.undo = []action{
		{kind: actionInsertRune, pos: Cursor{Row: 0, Col: 0}, r: 'h', group: 1},
	}

	if err := e.SaveUndoHistory(); err != nil {
		t.Fatalf("SaveUndoHistory returned error: %v", err)
	}
	if store.savedPath != "/tmp/file.txt" {
		t.Fatalf("saved path = %q, want %q", store.savedPath, "/tmp/file.txt")
	}

	decoder := json.NewDecoder(bytes.NewReader(store.savedData))
	var header undoHistoryHeader
	if err := decoder.Decode(&header); err != nil {
		t.Fatalf("decode header: %v", err)
	}
	if header.Version != 1 {
		t.Fatalf("header version = %d, want 1", header.Version)
	}

	var record actionJSON
	if err := decoder.Decode(&record); err != nil {
		t.Fatalf("decode action: %v", err)
	}
	if record.Kind != int(actionInsertRune) || record.PosRow != 0 || record.PosCol != 0 || record.R != 'h' {
		t.Fatalf("saved action = %#v, want insert-rune at 0:0 with 'h'", record)
	}
}

func TestLoadUndoHistoryUsesRuntimeStore(t *testing.T) {
	e := newTestEditor("hello")
	store := &testUndoStore{loadData: map[string][]byte{}}
	e.SetUndoStore(store)
	e.document.filename = "/tmp/file.txt"

	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	if err := encoder.Encode(undoHistoryHeader{Version: 1, Mtime: 0}); err != nil {
		t.Fatalf("encode header: %v", err)
	}
	if err := encoder.Encode(actionToJSON(action{
		kind:  actionInsertRune,
		pos:   Cursor{Row: 1, Col: 2},
		r:     'x',
		group: 7,
	})); err != nil {
		t.Fatalf("encode action: %v", err)
	}
	store.loadData["/tmp/file.txt"] = data.Bytes()

	if err := e.LoadUndoHistory(); err != nil {
		t.Fatalf("LoadUndoHistory returned error: %v", err)
	}
	if len(e.undo) != 1 {
		t.Fatalf("undo len = %d, want 1", len(e.undo))
	}
	if e.undo[0].kind != actionInsertRune || e.undo[0].pos.Row != 1 || e.undo[0].pos.Col != 2 || e.undo[0].r != 'x' || e.undo[0].group != 7 {
		t.Fatalf("loaded undo action = %#v, want insert-rune at 1:2 with group 7", e.undo[0])
	}
}

func TestClearUndoHistoryUsesRuntimeStore(t *testing.T) {
	e := newTestEditor("hello")
	store := &testUndoStore{}
	e.SetUndoStore(store)
	e.document.filename = "/tmp/file.txt"

	if err := e.ClearUndoHistory(); err != nil {
		t.Fatalf("ClearUndoHistory returned error: %v", err)
	}
	if store.removedPath != "/tmp/file.txt" {
		t.Fatalf("removed path = %q, want %q", store.removedPath, "/tmp/file.txt")
	}
}
