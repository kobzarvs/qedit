package app

import (
	"os"
	"runtime"
	"time"

	"github.com/gdamore/tcell/v2"
	sitter "github.com/smacker/go-tree-sitter"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/editor"
	"github.com/kobzarvs/qedit/internal/gitinfo"
	"github.com/kobzarvs/qedit/internal/lsp"
	"github.com/kobzarvs/qedit/internal/platform/keyboard"
	"github.com/kobzarvs/qedit/internal/treesitter"
)

// App is the top-level runtime for qedit.
type App struct {
	args []string
}

func New(args []string) *App {
	return &App{args: args}
}

func (a *App) Run() error {
	runtime.LockOSThread()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	langs, err := config.LoadLanguages()
	if err != nil {
		return err
	}

	s, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := s.Init(); err != nil {
		return err
	}
	s.EnableMouse()
	defer s.Fini()

	ls := lsp.NewManager(langs)
	if err := ls.Start(); err != nil {
		return err
	}
	defer func() { _ = ls.Stop() }()

	ts := treesitter.New(langs)
	if err := ts.Start(); err != nil {
		return err
	}
	defer func() { _ = ts.Stop() }()

	stopLayout := make(chan struct{})
	defer close(stopLayout)
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopLayout:
				return
			case <-ticker.C:
				_ = s.PostEvent(tcell.NewEventInterrupt(nil))
			}
		}
	}()

	const maxHighlightBytes = 8 << 20
	ed := editor.New(cfg)
	ed.LoadCmdHistory()
	gitPath := ""
	var openPath string
	var langName string
	highlightEnabled := true
	highlightExpected := false
	if len(a.args) > 0 {
		openPath = a.args[0]
		if err := ed.OpenFile(openPath); err != nil {
			return err
		}
		gitPath = openPath
		if info, err := os.Stat(openPath); err == nil && info.Size() > maxHighlightBytes {
			highlightEnabled = false
		}
		content := ed.Content()
		ls.OpenFile(openPath, content)
		if highlightEnabled {
			if lang := langs.Match(openPath); lang != nil {
				langName = lang.Name
			}
		}
		highlightExpected = highlightEnabled && langName != ""
	}
	if gitPath == "" {
		if cwd, err := os.Getwd(); err == nil {
			gitPath = cwd
		}
	}

	lastLayoutRaw := keyboard.CurrentLayoutRaw()
	ed.SetKeyboardLayout(keyboard.CurrentLayout())
	ed.SetGitBranch(gitinfo.Branch(gitPath))
	lastGitCheck := time.Now()
	lastChangeTick := ed.ChangeTick()
	lastHighlightStart := -1
	lastHighlightEnd := -1
	if openPath != "" && highlightEnabled && langName != "" {
		if ts.ParseSync(openPath, langName, ed.Content()) {
			_, h := s.Size()
			viewHeight := h - 2
			if viewHeight < 0 {
				viewHeight = 0
			}
			end := viewHeight - 1
			if end < 0 {
				end = 0
			}
			lineCount := ed.LineCount()
			if lineCount > 0 && end >= lineCount {
				end = lineCount - 1
			}
			spans := ts.Highlights(openPath, 0, end)
			if spans != nil {
				editorSpans := make(map[int][]editor.HighlightSpan, len(spans))
				for line, lineSpans := range spans {
					dst := make([]editor.HighlightSpan, len(lineSpans))
					for i, span := range lineSpans {
						dst[i] = editor.HighlightSpan{
							StartCol: span.StartCol,
							EndCol:   span.EndCol,
							Kind:     span.Kind,
						}
					}
					editorSpans[line] = dst
				}
				ed.SetHighlights(0, end, editorSpans)
				lastHighlightStart = 0
				lastHighlightEnd = end
			}
		} else {
			highlightExpected = false
		}
	}
	ed.Render(s)
	for {
		ev := s.PollEvent()
		isMouseScroll := false
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if ed.HandleKey(ev) {
				return nil
			}
		case *tcell.EventMouse:
			ed.HandleMouse(ev)
			isMouseScroll = true
		case *tcell.EventResize:
			s.Sync()
		case *tcell.EventInterrupt:
			// Layout updates are handled below.
		}
		if !isMouseScroll {
			ed.UpdateScroll()
		}
		if ed.ConsumeBranchPickerRequest() {
			if gitPath == "" {
				ed.SetStatusMessage("not a git repository")
			} else {
				branches, current, err := gitinfo.ListBranches(gitPath)
				if err != nil {
					ed.SetStatusMessage(err.Error())
				} else {
					ed.ShowBranchPicker(branches, current)
				}
			}
		}
		if branch, ok := ed.ConsumeBranchSelection(); ok {
			if gitPath == "" {
				ed.SetStatusMessage("not a git repository")
			} else if err := gitinfo.Checkout(gitPath, branch); err != nil {
				ed.SetStatusMessage(err.Error())
			} else {
				ed.SetGitBranch(branch)
				ed.SetStatusMessage("checked out " + branch)
			}
		}
		if openPath != "" && highlightEnabled && langName != "" {
			tick := ed.ChangeTick()
			changed := tick != lastChangeTick
			if changed {
				lastChangeTick = tick
				if edit, ok := ed.ConsumeLastEdit(); ok {
					tsEdit := sitter.EditInput{
						StartIndex:  uint32(edit.StartByte),
						OldEndIndex: uint32(edit.OldEndByte),
						NewEndIndex: uint32(edit.NewEndByte),
						StartPoint: sitter.Point{
							Row:    uint32(edit.StartRow),
							Column: uint32(edit.StartColBytes),
						},
						OldEndPoint: sitter.Point{
							Row:    uint32(edit.OldEndRow),
							Column: uint32(edit.OldEndColBytes),
						},
						NewEndPoint: sitter.Point{
							Row:    uint32(edit.NewEndRow),
							Column: uint32(edit.NewEndColBytes),
						},
					}
					ts.ParseSyncEdit(openPath, langName, ed.Content(), &tsEdit)
				} else {
					ts.ParseSync(openPath, langName, ed.Content())
				}
			}
			start, end := ed.VisibleRange()
			if changed || start != lastHighlightStart || end != lastHighlightEnd {
				spans := ts.Highlights(openPath, start, end)
				if spans != nil {
					editorSpans := make(map[int][]editor.HighlightSpan, len(spans))
					for line, lineSpans := range spans {
						dst := make([]editor.HighlightSpan, len(lineSpans))
						for i, span := range lineSpans {
							dst[i] = editor.HighlightSpan{
								StartCol: span.StartCol,
								EndCol:   span.EndCol,
								Kind:     span.Kind,
							}
						}
						editorSpans[line] = dst
					}
					ed.SetHighlights(start, end, editorSpans)
					lastHighlightStart = start
					lastHighlightEnd = end
				} else {
					ed.SetHighlights(-1, -1, nil)
					lastHighlightStart = -1
					lastHighlightEnd = -1
				}
			}
		} else if openPath != "" {
			ed.SetHighlights(-1, -1, nil)
		}
		layoutRaw := keyboard.CurrentLayoutRaw()
		if layoutRaw != lastLayoutRaw {
			lastLayoutRaw = layoutRaw
			ed.SetKeyboardLayout(keyboard.CurrentLayout())
		}
		if gitPath != "" && time.Since(lastGitCheck) > 2*time.Second {
			lastGitCheck = time.Now()
			ed.SetGitBranch(gitinfo.Branch(gitPath))
		}
		if highlightExpected && !ed.HasHighlights() {
			continue
		}
		ed.Render(s)
	}
}
