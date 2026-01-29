package treesitter

import (
	"github.com/kobzarvs/qedit/internal/config"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

type Event struct {
	Kind string
	Path string
}

type Engine struct {
	langs   config.Languages
	parsers map[string]*sitter.Parser
	trees   map[string]*sitter.Tree
	reqCh   chan parseRequest
	events  chan Event
	stopCh  chan struct{}
}

type parseRequest struct {
	path     string
	language string
	text     string
}

func New(langs config.Languages) *Engine {
	return &Engine{
		langs:   langs,
		parsers: make(map[string]*sitter.Parser),
		trees:   make(map[string]*sitter.Tree),
		reqCh:   make(chan parseRequest, 8),
		events:  make(chan Event, 16),
		stopCh:  make(chan struct{}),
	}
}

func (e *Engine) Start() error {
	p := sitter.NewParser()
	p.SetLanguage(golang.GetLanguage())
	e.parsers["go"] = p

	go e.loop()
	return nil
}

func (e *Engine) Stop() error {
	select {
	case <-e.stopCh:
		return nil
	default:
		close(e.stopCh)
		return nil
	}
}

func (e *Engine) Events() <-chan Event {
	return e.events
}

func (e *Engine) OpenFile(path, text string) {
	lang := e.langs.Match(path)
	if lang == nil {
		return
	}
	e.Parse(path, lang.Name, text)
}

func (e *Engine) Parse(path, language, text string) {
	select {
	case e.reqCh <- parseRequest{path: path, language: language, text: text}:
	default:
	}
}

func (e *Engine) loop() {
	for {
		select {
		case <-e.stopCh:
			return
		case req := <-e.reqCh:
			parser, ok := e.parsers[req.language]
			if !ok {
				continue
			}
			prev := e.trees[req.path]
			tree := parser.Parse(prev, []byte(req.text))
			e.trees[req.path] = tree
			e.sendEvent("parsed", req.path)
		}
	}
}

func (e *Engine) sendEvent(kind, path string) {
	select {
	case e.events <- Event{Kind: kind, Path: path}:
	default:
	}
}
