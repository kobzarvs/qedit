package app

import (
	"os"
	"runtime"
	"time"

	"github.com/gdamore/tcell/v2"

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
	defer s.Fini()

	ls := lsp.NewManager(langs)
	if err := ls.Start(); err != nil {
		return err
	}
	defer ls.Stop()

	ts := treesitter.New(langs)
	if err := ts.Start(); err != nil {
		return err
	}
	defer ts.Stop()

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
				if err := s.PostEvent(tcell.NewEventInterrupt(nil)); err != nil {
					s.PostEventWait(tcell.NewEventInterrupt(nil))
				}
			}
		}
	}()

	ed := editor.New(cfg)
	gitPath := ""
	if len(a.args) > 0 {
		if err := ed.OpenFile(a.args[0]); err != nil {
			return err
		}
		gitPath = a.args[0]
		content := ed.Content()
		ls.OpenFile(a.args[0], content)
		ts.OpenFile(a.args[0], content)
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
	ed.Render(s)
	for {
		ev := s.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if ed.HandleKey(ev) {
				return nil
			}
		case *tcell.EventResize:
			s.Sync()
		case *tcell.EventInterrupt:
			// Layout updates are handled below.
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
		layoutRaw := keyboard.CurrentLayoutRaw()
		if layoutRaw != lastLayoutRaw {
			lastLayoutRaw = layoutRaw
			ed.SetKeyboardLayout(keyboard.CurrentLayout())
		}
		if gitPath != "" && time.Since(lastGitCheck) > 2*time.Second {
			lastGitCheck = time.Now()
			ed.SetGitBranch(gitinfo.Branch(gitPath))
		}
		ed.Render(s)
	}
}
