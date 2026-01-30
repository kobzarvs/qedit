package treesitter

import (
	"context"
	"math"
	"sync"

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
	queries map[string]*sitter.Query
	sources map[string][]byte
	reqCh   chan parseRequest
	events  chan Event
	stopCh  chan struct{}
	mu      sync.RWMutex
}

type HighlightSpan struct {
	StartCol int
	EndCol   int
	Kind     string
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
		queries: make(map[string]*sitter.Query),
		sources: make(map[string][]byte),
		reqCh:   make(chan parseRequest, 8),
		events:  make(chan Event, 16),
		stopCh:  make(chan struct{}),
	}
}

func (e *Engine) Start() error {
	p := sitter.NewParser()
	p.SetLanguage(golang.GetLanguage())
	e.parsers["go"] = p
	query, err := sitter.NewQuery([]byte(goHighlightQuery), golang.GetLanguage())
	if err != nil {
		return err
	}
	e.queries["go"] = query

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
			e.mu.Lock()
			tree, _ := parser.ParseCtx(context.Background(), nil, []byte(req.text))
			e.trees[req.path] = tree
			e.sources[req.path] = []byte(req.text)
			e.mu.Unlock()
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

func (e *Engine) ParseSync(path, language, text string) bool {
	return e.parseSync(path, language, text, nil)
}

func (e *Engine) ParseSyncEdit(path, language, text string, edit *sitter.EditInput) bool {
	return e.parseSync(path, language, text, edit)
}

func (e *Engine) parseSync(path, language, text string, edit *sitter.EditInput) bool {
	lang := language
	if lang == "" {
		if detected := e.langs.Match(path); detected != nil {
			lang = detected.Name
		}
	}
	if lang == "" {
		return false
	}
	var tsLang *sitter.Language
	switch lang {
	case "go":
		tsLang = golang.GetLanguage()
	default:
		return false
	}
	e.mu.Lock()
	parser := e.parsers[lang]
	if parser == nil {
		parser = sitter.NewParser()
		parser.SetLanguage(tsLang)
		e.parsers[lang] = parser
	}
	prev := e.trees[path]
	if edit == nil {
		prev = nil
	}
	if prev != nil && edit != nil {
		prev.Edit(*edit)
	}
	tree, _ := parser.ParseCtx(context.Background(), prev, []byte(text))
	e.trees[path] = tree
	e.sources[path] = []byte(text)
	e.mu.Unlock()
	e.sendEvent("parsed", path)
	return true
}

func (e *Engine) Highlights(path string, startLine, endLine int) map[int][]HighlightSpan {
	if startLine < 0 || endLine < startLine {
		return nil
	}
	lang := e.langs.Match(path)
	if lang == nil {
		return nil
	}
	e.mu.RLock()
	query, ok := e.queries[lang.Name]
	if !ok || query == nil {
		e.mu.RUnlock()
		return nil
	}
	tree := e.trees[path]
	if tree == nil {
		e.mu.RUnlock()
		return nil
	}
	source := e.sources[path]
	e.mu.RUnlock()
	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.SetPointRange(
		sitter.Point{Row: uint32(startLine), Column: 0},
		sitter.Point{Row: uint32(endLine + 1), Column: 0},
	)
	cursor.Exec(query, tree.RootNode())

	out := make(map[int][]HighlightSpan)
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		if source != nil {
			match = cursor.FilterPredicates(match, source)
			if match == nil {
				continue
			}
		}
		for _, capture := range match.Captures {
			kind := query.CaptureNameForId(capture.Index)
			node := capture.Node
			start := node.StartPoint()
			end := node.EndPoint()
			if int(end.Row) < startLine || int(start.Row) > endLine {
				continue
			}
			startRow := int(start.Row)
			endRow := int(end.Row)
			for row := startRow; row <= endRow; row++ {
				if row < startLine || row > endLine {
					continue
				}
				startCol := 0
				endCol := int(math.MaxInt32)
				if row == startRow {
					startCol = int(start.Column)
				}
				if row == endRow {
					endCol = int(end.Column)
				}
				out[row] = append(out[row], HighlightSpan{
					StartCol: startCol,
					EndCol:   endCol,
					Kind:     kind,
				})
			}
		}
	}
	return out
}

// NodeRange represents a syntax node's position range
type NodeRange struct {
	StartRow int
	StartCol int
	EndRow   int
	EndCol   int
}

// GetNodeStackAt returns a stack of node ranges at the given position,
// from innermost to outermost (root). Used for expand/shrink selection.
func (e *Engine) GetNodeStackAt(path string, row, col int) []NodeRange {
	e.mu.RLock()
	tree := e.trees[path]
	e.mu.RUnlock()

	if tree == nil {
		return nil
	}

	root := tree.RootNode()
	if root == nil {
		return nil
	}

	// Find the deepest node containing this position
	point := sitter.Point{Row: uint32(row), Column: uint32(col)}
	node := root.NamedDescendantForPointRange(point, point)
	if node == nil {
		return nil
	}

	// Build stack from innermost to outermost
	var stack []NodeRange
	for node != nil {
		start := node.StartPoint()
		end := node.EndPoint()
		nr := NodeRange{
			StartRow: int(start.Row),
			StartCol: int(start.Column),
			EndRow:   int(end.Row),
			EndCol:   int(end.Column),
		}
		// Only add if different from previous (avoid duplicates)
		if len(stack) == 0 || stack[len(stack)-1] != nr {
			stack = append(stack, nr)
		}
		node = node.Parent()
	}

	return stack
}

const goHighlightQuery = `
((comment) @comment)
((interpreted_string_literal) @string)
((raw_string_literal) @string)
((rune_literal) @string)
((escape_sequence) @string)
((int_literal) @number)
((float_literal) @number)
((imaginary_literal) @number)
[
  "break" "case" "chan" "const" "continue" "default" "defer" "else"
  "fallthrough" "for" "func" "go" "goto" "if" "import" "interface"
  "map" "package" "range" "return" "select" "struct" "switch"
  "type" "var"
] @keyword
((nil) @constant)
((true) @constant)
((false) @constant)
((iota) @constant)
((identifier) @type (#match? @type "^(bool|byte|rune|string|int|int8|int16|int32|int64|uint|uint8|uint16|uint32|uint64|uintptr|float32|float64|complex64|complex128|error|any|comparable)$"))
((identifier) @builtin (#match? @builtin "^(append|cap|clear|close|complex|copy|delete|imag|len|make|max|min|new|panic|print|println|real|recover)$"))
((const_spec name: (identifier) @constant))
((type_spec name: (type_identifier) @type))
((type_identifier) @type)
((package_identifier) @type)
((type_parameter_declaration (identifier) @type))
((function_declaration name: (identifier) @function))
((method_declaration name: (field_identifier) @function))
((method_elem (field_identifier) @function))
((call_expression function: (identifier) @function))
((call_expression function: (selector_expression field: (field_identifier) @function)))
((selector_expression field: (field_identifier) @field))
((field_identifier) @field)
((parameter_declaration (identifier) @parameter))
((variadic_parameter_declaration (identifier) @parameter))
((label_name) @keyword)
((blank_identifier) @variable)
((identifier) @variable)
[
  "+" "-" "*" "/" "%" "==" "!=" "<=" ">=" "<" ">" "=" ":=" "&&" "||"
  "!" "&" "|" "^" "<<" ">>" "&^" "+=" "-=" "*=" "/=" "%=" "&=" "|="
  "^=" "<<=" ">>=" "&^=" "<-" "++" "--" "..."
] @operator
[
  "." "," ";" ":" "(" ")" "[" "]" "{" "}"
] @punctuation
`
