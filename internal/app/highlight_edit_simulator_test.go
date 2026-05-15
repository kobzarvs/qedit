package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/integrations"
	"github.com/kobzarvs/qedit/internal/treesitter"
)

type highlightEditAction struct {
	name string
	run  func(*highlightEditSimulator)
}

type highlightEditSimulator struct {
	t         *testing.T
	ed        *editor.Editor
	ts        *treesitter.Engine
	path      string
	langName  string
	enabled   bool
	screen    *appTestScreen
	state     editorRuntimeState
	lastTick  uint64
	lastStart int
	lastEnd   int
}

func newHighlightEditSimulator(t *testing.T, path, content string) *highlightEditSimulator {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	langs := config.Languages{
		Languages: []config.Language{
			{Name: "javascript", FileTypes: []string{"js"}},
		},
	}
	ts := treesitter.New(langs)
	if err := ts.Start(); err != nil {
		t.Fatalf("start treesitter: %v", err)
	}
	t.Cleanup(func() { _ = ts.Stop() })

	ed := editor.New(editor.Options{
		Profile:     editor.BehaviorProfileBasic,
		LineNumbers: "off",
	})
	ed.SetStyles(appTestEditorStyles())
	state, err := openRuntimeFile(ed, nil, nil, ts, langs, integrations.FileStore{}, path, 8<<20)
	if err != nil {
		t.Fatalf("openRuntimeFile returned error: %v", err)
	}
	if !ed.HugeFileMode() {
		t.Fatalf("expected huge mode for long-line edit simulator")
	}
	if !state.highlightEnabled || !state.highlightExpected || state.langName != "javascript" {
		t.Fatalf("unexpected highlight state: enabled=%v expected=%v lang=%q", state.highlightEnabled, state.highlightExpected, state.langName)
	}

	rtState := newEditorRuntimeState(ed)
	rtState.applyActiveFile(state)
	screen := newAppTestScreen(120, 12)
	ed.Render(screen)

	return &highlightEditSimulator{
		t:         t,
		ed:        ed,
		ts:        ts,
		path:      path,
		langName:  state.langName,
		enabled:   state.highlightEnabled,
		screen:    screen,
		state:     rtState,
		lastTick:  state.lastChangeTick,
		lastStart: state.lastHighlightStart,
		lastEnd:   state.lastHighlightEnd,
	}
}

func (s *highlightEditSimulator) jump(row, col int) {
	s.t.Helper()
	s.ed.JumpToLocation(row, col)
	s.ed.Render(s.screen)
}

func (s *highlightEditSimulator) insertRune(r rune) {
	s.t.Helper()
	s.ed.HandleKey(highlightTestKey{key: editor.KeyRune, r: r})
}

func (s *highlightEditSimulator) insertString(text string) {
	s.t.Helper()
	for _, r := range text {
		s.insertRune(r)
	}
}

func (s *highlightEditSimulator) insertNewline() {
	s.t.Helper()
	s.ed.HandleKey(highlightTestKey{key: editor.KeyEnter})
}

func (s *highlightEditSimulator) insertTab() {
	s.t.Helper()
	s.ed.HandleKey(highlightTestKey{key: editor.KeyTab})
}

func (s *highlightEditSimulator) backspace() {
	s.t.Helper()
	s.ed.HandleKey(highlightTestKey{key: editor.KeyBackspace})
}

func (s *highlightEditSimulator) run(actions ...highlightEditAction) {
	s.t.Helper()
	for _, action := range actions {
		action.run(s)
		s.waitForHighlightedRow(action.name, 1)
	}
}

func (s *highlightEditSimulator) waitForHighlightedRow(step string, screenRow int) {
	s.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.lastTick, s.lastStart, s.lastEnd = syncVisibleHighlights(s.ed, s.ts, &s.state, s.path, s.langName, s.enabled, s.lastTick, s.lastStart, s.lastEnd)
		s.ed.Render(s.screen)
		if s.screen.rowHasNonMainText(screenRow, appTestMainFG) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	s.t.Fatalf("timeout waiting for highlighted row %d after %s; dirty=%v parsed=%v jobActive=%v lastRange=(%d,%d)",
		screenRow, step, s.ed.IsDirty(), s.state.highlightParsed, s.state.highlightJobActive, s.lastStart, s.lastEnd)
}

type highlightTestKey struct {
	key  editor.Key
	r    rune
	mods editor.ModMask
}

func (k highlightTestKey) Key() editor.Key {
	return k.key
}

func (k highlightTestKey) Rune() rune {
	return k.r
}

func (k highlightTestKey) Modifiers() editor.ModMask {
	return k.mods
}

func TestHighlightEditSimulatorRehighlightsDirtyHugeFileAfterEachEdit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.js")
	line1 := "/*! For license information please see app.js.LICENSE.txt */"
	line2 := `(()=>{var e={"./assets/trayIcons/colorfulIcons.js":(e,t,n)=>{` +
		strings.Repeat(`var n=1;const s="abcdefgh";`, 8000) +
		`}});`
	content := line1 + "\n" + line2 + "\n//# sourceMappingURL=app.js.map\n"

	sim := newHighlightEditSimulator(t, path, content)
	sim.waitForHighlightedRow("initial parse", 1)
	sim.jump(1, 61)
	sim.run(
		highlightEditAction{
			name: "insert newline at column 61",
			run: func(s *highlightEditSimulator) {
				s.insertNewline()
			},
		},
		highlightEditAction{
			name: "insert one rune after split",
			run: func(s *highlightEditSimulator) {
				s.insertRune('x')
			},
		},
		highlightEditAction{
			name: "backspace inserted rune",
			run: func(s *highlightEditSimulator) {
				s.backspace()
			},
		},
		highlightEditAction{
			name: "insert tab on split line",
			run: func(s *highlightEditSimulator) {
				s.insertTab()
			},
		},
		highlightEditAction{
			name: "insert short text on split line",
			run: func(s *highlightEditSimulator) {
				s.insertString("var")
			},
		},
	)
}
