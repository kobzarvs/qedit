package app

import (
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/kobzarvs/qedit/internal/config"
	"github.com/kobzarvs/qedit/internal/lsp"
	"github.com/kobzarvs/qedit/internal/treesitter"
)

type appServices struct {
	screen     tcell.Screen
	ls         *lsp.Manager
	ts         *treesitter.Engine
	stopLayout chan struct{}
}

func newAppServices(langs config.Languages) (*appServices, error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return nil, err
	}
	if err := screen.Init(); err != nil {
		return nil, err
	}
	screen.EnableMouse()

	ls := lsp.NewManager(langs)
	if err := ls.Start(); err != nil {
		screen.Fini()
		return nil, err
	}

	ts := treesitter.New(langs)
	if err := ts.Start(); err != nil {
		_ = ls.Stop()
		screen.Fini()
		return nil, err
	}

	svc := &appServices{
		screen:     screen,
		ls:         ls,
		ts:         ts,
		stopLayout: make(chan struct{}),
	}
	go svc.layoutInterruptLoop()
	return svc, nil
}

func (s *appServices) layoutInterruptLoop() {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopLayout:
			return
		case <-ticker.C:
			_ = s.screen.PostEvent(tcell.NewEventInterrupt(nil))
		}
	}
}

func (s *appServices) Close() {
	close(s.stopLayout)
	_ = s.ts.Stop()
	_ = s.ls.Stop()
	s.screen.Fini()
}
