package treesitter

import (
	"context"
	"math"
	"regexp"
	"strings"
	"sync"

	"github.com/kobzarvs/qedit/internal/config"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/bash"
	"github.com/smacker/go-tree-sitter/golang"
	tree_sitter_markdown "github.com/smacker/go-tree-sitter/markdown/tree-sitter-markdown"
	tree_sitter_markdown_inline "github.com/smacker/go-tree-sitter/markdown/tree-sitter-markdown-inline"
	"github.com/smacker/go-tree-sitter/toml"
	"github.com/smacker/go-tree-sitter/yaml"
)

type Event struct {
	Kind string
	Path string
}

type Engine struct {
	langs         config.Languages
	parsers       map[string]*sitter.Parser
	trees         map[string]*sitter.Tree
	queries       map[string]*sitter.Query
	sources       map[string][]byte
	mdInlineQuery *sitter.Query
	reqCh         chan parseRequest
	events        chan Event
	stopCh        chan struct{}
	mu            sync.RWMutex
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
	// Initialize all supported languages
	languages := []struct {
		name  string
		lang  *sitter.Language
		query string
	}{
		{"go", golang.GetLanguage(), goHighlightQuery},
		{"markdown", tree_sitter_markdown.GetLanguage(), markdownBlockHighlightQuery},
		{"yaml", yaml.GetLanguage(), yamlHighlightQuery},
		{"toml", toml.GetLanguage(), tomlHighlightQuery},
		{"bash", bash.GetLanguage(), bashHighlightQuery},
	}

	for _, l := range languages {
		p := sitter.NewParser()
		p.SetLanguage(l.lang)
		e.parsers[l.name] = p

		query, err := sitter.NewQuery([]byte(l.query), l.lang)
		if err != nil {
			// Log error but continue with other languages
			continue
		}
		e.queries[l.name] = query
	}

	inlineQuery, err := sitter.NewQuery([]byte(markdownInlineHighlightQuery), tree_sitter_markdown_inline.GetLanguage())
	if err == nil {
		e.mdInlineQuery = inlineQuery
	}

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

	// For regex-based languages, just store the source
	switch lang.Name {
	case "json", "gitignore":
		e.mu.Lock()
		e.sources[path] = []byte(text)
		e.mu.Unlock()
		e.sendEvent("parsed", path)
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
			e.mu.RLock()
			parser, ok := e.parsers[req.language]
			e.mu.RUnlock()
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

	// For regex-based languages, just store the source
	switch lang {
	case "json", "gitignore":
		e.mu.Lock()
		e.sources[path] = []byte(text)
		e.mu.Unlock()
		e.sendEvent("parsed", path)
		return true
	}

	var tsLang *sitter.Language
	switch lang {
	case "go":
		tsLang = golang.GetLanguage()
	case "markdown":
		tsLang = tree_sitter_markdown.GetLanguage()
	case "yaml":
		tsLang = yaml.GetLanguage()
	case "toml":
		tsLang = toml.GetLanguage()
	case "bash":
		tsLang = bash.GetLanguage()
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

	// Try non-tree-sitter highlighting for languages without tree-sitter
	switch lang.Name {
	case "markdown":
		return e.markdownHighlights(path, startLine, endLine)
	case "json", "gitignore":
		e.mu.RLock()
		source := e.sources[path]
		e.mu.RUnlock()
		if source != nil {
			return e.regexHighlights(lang.Name, source, startLine, endLine)
		}
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
	return queryHighlights(query, tree, source, startLine, endLine)
}

func queryHighlights(query *sitter.Query, tree *sitter.Tree, source []byte, startLine, endLine int) map[int][]HighlightSpan {
	if query == nil || tree == nil {
		return nil
	}
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

func (e *Engine) markdownHighlights(path string, startLine, endLine int) map[int][]HighlightSpan {
	if startLine < 0 || endLine < startLine {
		return nil
	}
	e.mu.RLock()
	query := e.queries["markdown"]
	inlineQuery := e.mdInlineQuery
	tree := e.trees[path]
	source := e.sources[path]
	e.mu.RUnlock()

	if query == nil || tree == nil || source == nil {
		return map[int][]HighlightSpan{}
	}

	out := queryHighlights(query, tree, source, startLine, endLine)
	if out == nil {
		out = make(map[int][]HighlightSpan)
	}
	if inlineQuery == nil {
		return out
	}

	lines := strings.Split(string(source), "\n")
	if len(lines) == 0 {
		return out
	}
	if startLine < 0 {
		startLine = 0
	}
	if endLine >= len(lines) {
		endLine = len(lines) - 1
	}

	root := tree.RootNode()
	fenceBlocks := collectMarkdownFencedBlocks(root, source)
	tableRows := collectMarkdownTableRows(root)
	skipInline := make(map[int]bool, len(fenceBlocks)*2)
	for i := range fenceBlocks {
		block := fenceBlocks[i]
		if block.blockStartRow < 0 || block.blockEndRow < block.blockStartRow {
			continue
		}
		for row := block.blockStartRow; row <= block.blockEndRow; row++ {
			skipInline[row] = true
		}
	}

	inlineParser := sitter.NewParser()
	inlineParser.SetLanguage(tree_sitter_markdown_inline.GetLanguage())
	for row := startLine; row <= endLine && row < len(lines); row++ {
		if row < 0 {
			continue
		}
		if skipInline[row] {
			continue
		}
		line := lines[row]
		if line == "" {
			continue
		}
		inlineTree, _ := inlineParser.ParseCtx(context.Background(), nil, []byte(line))
		if inlineTree == nil {
			continue
		}
		lineSpans := queryHighlights(inlineQuery, inlineTree, []byte(line), 0, 0)
		if len(lineSpans) == 0 {
			continue
		}
		for offset, spans := range lineSpans {
			targetRow := row + offset
			if targetRow < startLine || targetRow > endLine {
				continue
			}
			out[targetRow] = append(out[targetRow], spans...)
		}
	}

	if len(fenceBlocks) > 0 {
		for i := range fenceBlocks {
			e.applyFencedBlockHighlights(out, fenceBlocks[i], lines, startLine, endLine)
		}
	}

	for row := startLine; row <= endLine && row < len(lines); row++ {
		if row < 0 {
			continue
		}
		if skipInline[row] {
			continue
		}
		line := lines[row]
		if line == "" {
			continue
		}
		if _, ok := out[row]; !ok {
			out[row] = []HighlightSpan{{StartCol: 0, EndCol: len([]rune(line)), Kind: "variable"}}
			continue
		}
		out[row] = append(out[row], HighlightSpan{StartCol: 0, EndCol: len([]rune(line)), Kind: "variable"})
	}

	for row := startLine; row <= endLine && row < len(lines); row++ {
		if row < 0 || skipInline[row] {
			continue
		}
		line := lines[row]
		isTableRow := tableRows[row] || isPipeTableRowFallback(line)
		if !isTableRow {
			continue
		}
		isSeparator := isPipeTableSeparatorLine(line)
		lineRunes := []rune(line)
		for idx, r := range lineRunes {
			if r == '|' || (isSeparator && (r == '-' || r == ':')) {
				out[row] = append(out[row], HighlightSpan{
					StartCol: idx,
					EndCol:   idx + 1,
					Kind:     "text",
				})
			}
		}
	}

	return out
}

type mdFenceBlock struct {
	lang            string
	blockStartRow   int
	blockEndRow     int
	contentStartRow int
	contentEndRow   int
	offsets         map[int]int
}

func collectMarkdownFencedBlocks(root *sitter.Node, source []byte) []mdFenceBlock {
	if root == nil {
		return nil
	}
	var blocks []mdFenceBlock
	stack := []*sitter.Node{root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n == nil {
			continue
		}
		if n.Type() == "fenced_code_block" {
			if block, ok := buildMarkdownFenceBlock(n, source); ok {
				blocks = append(blocks, block)
			}
		}
		childCount := int(n.NamedChildCount())
		for i := 0; i < childCount; i++ {
			child := n.NamedChild(i)
			if child != nil {
				stack = append(stack, child)
			}
		}
	}
	return blocks
}

func collectMarkdownTableRows(root *sitter.Node) map[int]bool {
	rows := map[int]bool{}
	if root == nil {
		return rows
	}
	stack := []*sitter.Node{root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n == nil {
			continue
		}
		if n.Type() == "pipe_table_row" || n.Type() == "pipe_table_delimiter_row" {
			start := int(n.StartPoint().Row)
			end := int(n.EndPoint().Row)
			for row := start; row <= end; row++ {
				rows[row] = true
			}
		}
		childCount := int(n.NamedChildCount())
		for i := 0; i < childCount; i++ {
			child := n.NamedChild(i)
			if child != nil {
				stack = append(stack, child)
			}
		}
	}
	return rows
}

func isPipeTableRowFallback(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if !strings.Contains(trimmed, "|") {
		return false
	}
	if strings.Count(trimmed, "|") < 2 {
		return false
	}
	return true
}

func isPipeTableSeparatorLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if !strings.Contains(trimmed, "|") || !strings.Contains(trimmed, "-") {
		return false
	}
	for _, r := range trimmed {
		switch r {
		case '|', '-', ':', ' ', '\t':
			continue
		default:
			return false
		}
	}
	return true
}

func buildMarkdownFenceBlock(node *sitter.Node, source []byte) (mdFenceBlock, bool) {
	block := mdFenceBlock{
		blockStartRow:   int(node.StartPoint().Row),
		blockEndRow:     int(node.EndPoint().Row),
		contentStartRow: -1,
		contentEndRow:   -1,
		offsets:         map[int]int{},
	}
	childCount := int(node.NamedChildCount())
	for i := 0; i < childCount; i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "info_string":
			if block.lang == "" {
				block.lang = extractFenceLang(child, source)
			}
		case "language":
			if block.lang == "" {
				block.lang = extractNodeText(child, source)
			}
		case "code_fence_content":
			start := child.StartPoint()
			end := child.EndPoint()
			row := int(start.Row)
			if _, ok := block.offsets[row]; !ok {
				block.offsets[row] = int(start.Column)
			}
			if block.contentStartRow == -1 || row < block.contentStartRow {
				block.contentStartRow = row
			}
			if block.contentEndRow == -1 || int(end.Row) > block.contentEndRow {
				block.contentEndRow = int(end.Row)
			}
		}
	}
	if block.contentStartRow < 0 || block.contentEndRow < block.contentStartRow {
		return mdFenceBlock{}, false
	}
	block.lang = normalizeFenceLang(block.lang)
	return block, true
}

func extractFenceLang(infoNode *sitter.Node, source []byte) string {
	if infoNode == nil {
		return ""
	}
	if langNode := findNamedChild(infoNode, "language"); langNode != nil {
		return extractNodeText(langNode, source)
	}
	text := extractNodeText(infoNode, source)
	if text == "" {
		return ""
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func findNamedChild(node *sitter.Node, kind string) *sitter.Node {
	if node == nil {
		return nil
	}
	childCount := int(node.NamedChildCount())
	for i := 0; i < childCount; i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type() == kind {
			return child
		}
	}
	return nil
}

func extractNodeText(node *sitter.Node, source []byte) string {
	if node == nil || source == nil {
		return ""
	}
	start := int(node.StartByte())
	end := int(node.EndByte())
	if start < 0 || end < start || end > len(source) {
		return ""
	}
	return strings.TrimSpace(string(source[start:end]))
}

func normalizeFenceLang(info string) string {
	if info == "" {
		return ""
	}
	s := strings.TrimSpace(info)
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	s = strings.TrimPrefix(s, ".")
	s = strings.ToLower(s)
	switch s {
	case "golang":
		return "go"
	case "yml":
		return "yaml"
	case "shell", "sh", "zsh":
		return "bash"
	case "jsonc":
		return "json"
	default:
		return s
	}
}

func (e *Engine) applyFencedBlockHighlights(out map[int][]HighlightSpan, block mdFenceBlock, lines []string, startLine, endLine int) {
	if block.contentStartRow < 0 || block.contentEndRow < block.contentStartRow {
		return
	}
	if endLine < block.contentStartRow || startLine > block.contentEndRow {
		return
	}
	if block.contentStartRow >= len(lines) {
		return
	}
	contentEnd := block.contentEndRow
	if contentEnd >= len(lines) {
		contentEnd = len(lines) - 1
	}
	if contentEnd < block.contentStartRow {
		return
	}

	lineCount := contentEnd - block.contentStartRow + 1
	contentLines := make([]string, 0, lineCount)
	offsets := make([]int, lineCount)
	for row := block.contentStartRow; row <= contentEnd; row++ {
		offset := block.offsets[row]
		line := lines[row]
		lineBytes := []byte(line)
		if offset > len(lineBytes) {
			offset = len(lineBytes)
		}
		offsets[row-block.contentStartRow] = offset
		contentLines = append(contentLines, string(lineBytes[offset:]))
	}

	lang := block.lang
	if lang == "" {
		addFenceFallback(out, block, offsets, contentLines, startLine, endLine, "comment")
		addAsciiTableBorders(out, block, offsets, contentLines, startLine, endLine)
		return
	}
	switch lang {
	case "json":
		for idx, text := range contentLines {
			globalRow := block.contentStartRow + idx
			if globalRow < startLine || globalRow > endLine {
				continue
			}
			spans := e.highlightJSONLine(text)
			if len(spans) == 0 {
				continue
			}
			offset := offsets[idx]
			for _, span := range spans {
				out[globalRow] = append(out[globalRow], HighlightSpan{
					StartCol: span.StartCol + offset,
					EndCol:   span.EndCol + offset,
					Kind:     span.Kind,
				})
			}
		}
		return
	case "gitignore":
		for idx, text := range contentLines {
			globalRow := block.contentStartRow + idx
			if globalRow < startLine || globalRow > endLine {
				continue
			}
			spans := e.highlightGitignoreLine(text)
			if len(spans) == 0 {
				continue
			}
			offset := offsets[idx]
			for _, span := range spans {
				out[globalRow] = append(out[globalRow], HighlightSpan{
					StartCol: span.StartCol + offset,
					EndCol:   span.EndCol + offset,
					Kind:     span.Kind,
				})
			}
		}
		return
	}

	query := e.queries[lang]
	tsLang := tsLanguageForName(lang)
	if query == nil || tsLang == nil {
		addFenceFallback(out, block, offsets, contentLines, startLine, endLine, "string")
		return
	}
	text := strings.Join(contentLines, "\n")
	parser := sitter.NewParser()
	parser.SetLanguage(tsLang)
	tree, _ := parser.ParseCtx(context.Background(), nil, []byte(text))
	if tree == nil {
		addFenceFallback(out, block, offsets, contentLines, startLine, endLine, "string")
		return
	}
	spans := queryHighlights(query, tree, []byte(text), 0, lineCount-1)
	for line, lineSpans := range spans {
		globalRow := block.contentStartRow + line
		if globalRow < startLine || globalRow > endLine {
			continue
		}
		offset := offsets[line]
		for _, span := range lineSpans {
			out[globalRow] = append(out[globalRow], HighlightSpan{
				StartCol: span.StartCol + offset,
				EndCol:   span.EndCol + offset,
				Kind:     span.Kind,
			})
		}
	}
}

func addAsciiTableBorders(out map[int][]HighlightSpan, block mdFenceBlock, offsets []int, contentLines []string, startLine, endLine int) {
	for idx, text := range contentLines {
		globalRow := block.contentStartRow + idx
		if globalRow < startLine || globalRow > endLine {
			continue
		}
		if text == "" {
			continue
		}
		offset := offsets[idx]
		col := 0
		for _, r := range []rune(text) {
			if r == '|' || r == '+' || r == '-' || r == '=' {
				out[globalRow] = append(out[globalRow], HighlightSpan{
					StartCol: offset + col,
					EndCol:   offset + col + 1,
					Kind:     "text",
				})
			}
			col++
		}
	}
}

func addFenceFallback(out map[int][]HighlightSpan, block mdFenceBlock, offsets []int, contentLines []string, startLine, endLine int, kind string) {
	for idx, text := range contentLines {
		globalRow := block.contentStartRow + idx
		if globalRow < startLine || globalRow > endLine {
			continue
		}
		if text == "" {
			continue
		}
		offset := offsets[idx]
		out[globalRow] = append(out[globalRow], HighlightSpan{
			StartCol: offset,
			EndCol:   offset + len([]rune(text)),
			Kind:     kind,
		})
	}
}

func tsLanguageForName(name string) *sitter.Language {
	switch name {
	case "go":
		return golang.GetLanguage()
	case "yaml":
		return yaml.GetLanguage()
	case "toml":
		return toml.GetLanguage()
	case "bash":
		return bash.GetLanguage()
	default:
		return nil
	}
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

const yamlHighlightQuery = `
((comment) @comment)
((string_scalar) @string)
((double_quote_scalar) @string)
((single_quote_scalar) @string)
((integer_scalar) @number)
((float_scalar) @number)
((null_scalar) @constant)
((boolean_scalar) @constant)
((block_mapping_pair key: (_) @field))
((flow_pair key: (_) @field))
((anchor_name) @keyword)
((alias_name) @keyword)
((tag) @type)
["," ":" "-" "[" "]" "{" "}" ">" "|" "*" "&"] @punctuation
`

const tomlHighlightQuery = `
((comment) @comment)
((string) @string)
((integer) @number)
((float) @number)
((boolean) @constant)
((local_date) @string)
((local_time) @string)
((local_date_time) @string)
((offset_date_time) @string)
((bare_key) @field)
((quoted_key) @field)
((table (bare_key) @type))
((table (quoted_key) @type))
((table (dotted_key) @type))
((table_array_element (bare_key) @type))
((table_array_element (quoted_key) @type))
((table_array_element (dotted_key) @type))
["=" "." "," "[" "]" "[[" "]]" "{" "}"] @punctuation
`

const bashHighlightQuery = `
((comment) @comment)
((string) @string)
((raw_string) @string)
((heredoc_body) @string)
((number) @number)
((variable_name) @variable)
((special_variable_name) @variable)
((command_name) @function)
((function_definition name: (word) @function))
[
  "if" "then" "else" "elif" "fi" "case" "esac" "for" "while" "until"
  "do" "done" "in" "function" "select" "return" "exit" "break" "continue"
  "local" "export" "readonly" "declare" "typeset" "unset"
] @keyword
["$" "${" "}" "(" ")" "((" "))" "[" "]" "[[" "]]" "{" "}" ";" ";;" "&&" "||" "|" "&" "<" ">" ">>" "<<" "<<<"] @operator
`

const markdownBlockHighlightQuery = `
(atx_heading) @keyword
(setext_heading) @keyword
(thematic_break) @comment
(block_quote_marker) @comment
(list_marker_plus) @keyword
(list_marker_minus) @keyword
(list_marker_star) @keyword
(list_marker_dot) @keyword
(list_marker_parenthesis) @keyword
(task_list_marker_checked) @constant
(task_list_marker_unchecked) @constant
(fenced_code_block_delimiter) @string
(indented_code_block) @string
(info_string) @comment
(language) @type
(link_reference_definition) @function
(pipe_table_delimiter_row) @comment
(pipe_table_delimiter_cell) @comment
`

const markdownInlineHighlightQuery = `
(code_span) @string
(emphasis) @type
(strong_emphasis) @type
(strikethrough) @comment
(inline_link) @function
(full_reference_link) @function
(collapsed_reference_link) @function
(shortcut_link) @function
(image) @function
(link_text) @function
(link_destination) @string
(link_title) @string
(uri_autolink) @function
(email_autolink) @function
(html_tag) @type
`

// Regex patterns for languages without tree-sitter support
var (
	// JSON patterns
	jsonString = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
	jsonNumber = regexp.MustCompile(`-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?`)
	jsonBool   = regexp.MustCompile(`\b(true|false)\b`)
	jsonNull   = regexp.MustCompile(`\bnull\b`)

	// Gitignore patterns
	gitComment = regexp.MustCompile(`^#.*`)
	gitNegate  = regexp.MustCompile(`^!`)
	gitGlob    = regexp.MustCompile(`[*?]|\[.+?\]`)
)

// regexHighlights provides syntax highlighting using regex for languages without tree-sitter
func (e *Engine) regexHighlights(langName string, source []byte, startLine, endLine int) map[int][]HighlightSpan {
	lines := strings.Split(string(source), "\n")
	out := make(map[int][]HighlightSpan)

	for row := startLine; row <= endLine && row < len(lines); row++ {
		line := lines[row]
		switch langName {
		case "json":
			out[row] = e.highlightJSONLine(line)
		case "gitignore":
			out[row] = e.highlightGitignoreLine(line)
		}
	}

	return out
}

func (e *Engine) highlightJSONLine(line string) []HighlightSpan {
	var spans []HighlightSpan

	// Find all strings and determine if they are keys or values
	for _, loc := range jsonString.FindAllStringIndex(line, -1) {
		startRune := len([]rune(line[:loc[0]]))
		endRune := len([]rune(line[:loc[1]]))

		// Check if this is a key (followed by colon)
		rest := line[loc[1]:]
		trimmed := strings.TrimLeft(rest, " \t")
		if len(trimmed) > 0 && trimmed[0] == ':' {
			spans = append(spans, HighlightSpan{StartCol: startRune, EndCol: endRune, Kind: "field"})
		} else {
			spans = append(spans, HighlightSpan{StartCol: startRune, EndCol: endRune, Kind: "string"})
		}
	}

	// Numbers (only outside of strings)
	for _, loc := range jsonNumber.FindAllStringIndex(line, -1) {
		// Check if inside a string by looking for unescaped quotes before
		beforeStr := line[:loc[0]]
		quoteCount := strings.Count(beforeStr, `"`) - strings.Count(beforeStr, `\"`)
		if quoteCount%2 == 0 {
			startRune := len([]rune(line[:loc[0]]))
			endRune := len([]rune(line[:loc[1]]))
			spans = append(spans, HighlightSpan{StartCol: startRune, EndCol: endRune, Kind: "number"})
		}
	}

	// Booleans
	for _, loc := range jsonBool.FindAllStringIndex(line, -1) {
		beforeStr := line[:loc[0]]
		quoteCount := strings.Count(beforeStr, `"`) - strings.Count(beforeStr, `\"`)
		if quoteCount%2 == 0 {
			startRune := len([]rune(line[:loc[0]]))
			endRune := len([]rune(line[:loc[1]]))
			spans = append(spans, HighlightSpan{StartCol: startRune, EndCol: endRune, Kind: "constant"})
		}
	}

	// Null
	for _, loc := range jsonNull.FindAllStringIndex(line, -1) {
		beforeStr := line[:loc[0]]
		quoteCount := strings.Count(beforeStr, `"`) - strings.Count(beforeStr, `\"`)
		if quoteCount%2 == 0 {
			startRune := len([]rune(line[:loc[0]]))
			endRune := len([]rune(line[:loc[1]]))
			spans = append(spans, HighlightSpan{StartCol: startRune, EndCol: endRune, Kind: "constant"})
		}
	}

	return spans
}

func (e *Engine) highlightGitignoreLine(line string) []HighlightSpan {
	lineLen := len([]rune(line))
	if lineLen == 0 {
		return nil
	}

	// Comments
	if gitComment.MatchString(line) {
		return []HighlightSpan{{StartCol: 0, EndCol: lineLen, Kind: "comment"}}
	}

	var spans []HighlightSpan

	// Negation
	if loc := gitNegate.FindStringIndex(line); loc != nil {
		spans = append(spans, HighlightSpan{StartCol: 0, EndCol: 1, Kind: "keyword"})
	}

	// Glob patterns
	for _, loc := range gitGlob.FindAllStringIndex(line, -1) {
		startRune := len([]rune(line[:loc[0]]))
		endRune := len([]rune(line[:loc[1]]))
		spans = append(spans, HighlightSpan{StartCol: startRune, EndCol: endRune, Kind: "operator"})
	}

	return spans
}
