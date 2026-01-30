package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kobzarvs/qedit/internal/config"
)

type Event struct {
	Kind    string
	Message string
}

type Manager struct {
	langs   config.Languages
	servers map[string]*server
	events  chan Event
	mu      sync.Mutex
}

func NewManager(langs config.Languages) *Manager {
	return &Manager{
		langs:   langs,
		servers: make(map[string]*server),
		events:  make(chan Event, 32),
	}
}

func (m *Manager) Start() error {
	return nil
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, srv := range m.servers {
		srv.stop()
	}
	return nil
}

func (m *Manager) Events() <-chan Event {
	return m.events
}

func (m *Manager) OpenFile(path, text string) {
	if path == "" {
		return
	}
	lang := m.langs.Match(path)
	if lang == nil || len(lang.LanguageServers) == 0 {
		return
	}
	serverName := lang.LanguageServers[0]
	serverCfg, ok := m.langs.LanguageServers[serverName]
	if !ok || serverCfg.Command == "" {
		return
	}

	root := findRoot(path, lang.Roots)
	srv, err := m.getServer(serverName, serverCfg, root)
	if err != nil {
		m.sendEvent("error", err.Error())
		return
	}
	srv.didOpen(path, lang.Name, text)
}

func (m *Manager) getServer(name string, cfg config.LanguageServer, root string) (*server, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if srv, ok := m.servers[name]; ok {
		return srv, nil
	}
	if root == "" {
		root = "."
	}
	cmd := exec.Command(cfg.Command, cfg.Args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	rootURI := fileURI(root)
	srv := &server{
		name:        name,
		cmd:         cmd,
		stdin:       stdin,
		reader:      bufio.NewReader(stdout),
		rootURI:     rootURI,
		events:      m.events,
		docs:        make(map[string]int),
		initID:      -1,
		nextID:      0,
		initialized: false,
		handlers:    make(map[int]chan json.RawMessage),
	}
	m.servers[name] = srv
	go srv.readLoop()
	if err := srv.initialize(); err != nil {
		return srv, err
	}
	return srv, nil
}

func (m *Manager) sendEvent(kind, msg string) {
	select {
	case m.events <- Event{Kind: kind, Message: msg}:
	default:
	}
}

// getServerForFile returns the LSP server for a given file path
func (m *Manager) getServerForFile(path string) (*server, error) {
	lang := m.langs.Match(path)
	if lang == nil || len(lang.LanguageServers) == 0 {
		return nil, errors.New("no language server for this file type")
	}
	serverName := lang.LanguageServers[0]
	serverCfg, ok := m.langs.LanguageServers[serverName]
	if !ok || serverCfg.Command == "" {
		return nil, errors.New("language server not configured")
	}
	root := findRoot(path, lang.Roots)
	return m.getServer(serverName, serverCfg, root)
}

// GotoDefinition returns locations of definitions for the symbol at the given position
func (m *Manager) GotoDefinition(path string, line, col int) ([]Location, error) {
	srv, err := m.getServerForFile(path)
	if err != nil {
		return nil, err
	}
	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: fileURI(path)},
		Position:     Position{Line: line, Character: col},
	}
	return srv.requestLocations("textDocument/definition", params)
}

// GotoDeclaration returns locations of declarations for the symbol at the given position
func (m *Manager) GotoDeclaration(path string, line, col int) ([]Location, error) {
	srv, err := m.getServerForFile(path)
	if err != nil {
		return nil, err
	}
	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: fileURI(path)},
		Position:     Position{Line: line, Character: col},
	}
	return srv.requestLocations("textDocument/declaration", params)
}

// GotoTypeDefinition returns locations of type definitions for the symbol at the given position
func (m *Manager) GotoTypeDefinition(path string, line, col int) ([]Location, error) {
	srv, err := m.getServerForFile(path)
	if err != nil {
		return nil, err
	}
	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: fileURI(path)},
		Position:     Position{Line: line, Character: col},
	}
	return srv.requestLocations("textDocument/typeDefinition", params)
}

// FindReferences returns all references to the symbol at the given position
func (m *Manager) FindReferences(path string, line, col int) ([]Location, error) {
	srv, err := m.getServerForFile(path)
	if err != nil {
		return nil, err
	}
	params := ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: fileURI(path)},
		Position:     Position{Line: line, Character: col},
		Context:      ReferenceContext{IncludeDeclaration: true},
	}
	return srv.requestLocations("textDocument/references", params)
}

// GotoImplementation returns locations of implementations for the symbol at the given position
func (m *Manager) GotoImplementation(path string, line, col int) ([]Location, error) {
	srv, err := m.getServerForFile(path)
	if err != nil {
		return nil, err
	}
	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: fileURI(path)},
		Position:     Position{Line: line, Character: col},
	}
	return srv.requestLocations("textDocument/implementation", params)
}

// Position represents a position in a text document (LSP spec)
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range represents a range in a text document (LSP spec)
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location represents a location inside a resource (LSP spec)
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// TextDocumentIdentifier identifies a text document
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// TextDocumentPositionParams is used for requests like definition, declaration, etc.
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// ReferenceContext is used for textDocument/references request
type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

// ReferenceParams is used for textDocument/references request
type ReferenceParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Context      ReferenceContext       `json:"context"`
}

type server struct {
	name        string
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	reader      *bufio.Reader
	rootURI     string
	events      chan Event
	mu          sync.Mutex
	nextID      int
	initID      int
	initialized bool
	pendingOpen []openRequest
	docs        map[string]int
	handlers    map[int]chan json.RawMessage // pending request handlers
}

type openRequest struct {
	uri        string
	languageID string
	text       string
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type initializeParams struct {
	ProcessID  int                    `json:"processId"`
	RootURI    string                 `json:"rootUri"`
	Capabilities map[string]any       `json:"capabilities"`
	ClientInfo map[string]string      `json:"clientInfo"`
}

type textDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type didOpenParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

func (s *server) initialize() error {
	params := initializeParams{
		ProcessID:   os.Getpid(),
		RootURI:     s.rootURI,
		Capabilities: map[string]any{},
		ClientInfo:  map[string]string{"name": "qedit"},
	}
	s.mu.Lock()
	s.nextID++
	id := s.nextID
	s.initID = id
	s.mu.Unlock()

	msg := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "initialize",
		Params:  params,
	}
	return s.send(msg)
}

func (s *server) didOpen(path, languageID, text string) {
	uri := fileURI(path)
	s.mu.Lock()
	if _, ok := s.docs[uri]; ok {
		s.mu.Unlock()
		return
	}
	if !s.initialized {
		s.pendingOpen = append(s.pendingOpen, openRequest{uri: uri, languageID: languageID, text: text})
		s.mu.Unlock()
		return
	}
	s.docs[uri] = 1
	s.mu.Unlock()
	_ = s.sendNotification("textDocument/didOpen", didOpenParams{
		TextDocument: textDocumentItem{
			URI:        uri,
			LanguageID: languageID,
			Version:    1,
			Text:       text,
		},
	})
}

func (s *server) readLoop() {
	for {
		msg, err := readMessage(s.reader)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				s.sendEvent("error", err.Error())
			}
			return
		}
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(msg, &envelope); err != nil {
			continue
		}
		if idRaw, ok := envelope["id"]; ok {
			var id int
			if err := json.Unmarshal(idRaw, &id); err == nil {
				s.handleResponse(id, envelope)
			}
			continue
		}
	}
}

func (s *server) handleResponse(id int, envelope map[string]json.RawMessage) {
	s.mu.Lock()
	// Check if there's a waiting handler for this response
	if ch, ok := s.handlers[id]; ok {
		delete(s.handlers, id)
		s.mu.Unlock()
		// Send the result (or error) to the waiting handler
		if result, ok := envelope["result"]; ok {
			ch <- result
		} else if errRaw, ok := envelope["error"]; ok {
			ch <- errRaw
		} else {
			ch <- nil
		}
		close(ch)
		return
	}

	// Handle initialize response
	if id != s.initID || s.initialized {
		s.mu.Unlock()
		return
	}
	s.initialized = true
	pending := append([]openRequest(nil), s.pendingOpen...)
	s.pendingOpen = nil
	s.mu.Unlock()

	_ = s.sendNotification("initialized", map[string]any{})
	for _, req := range pending {
		s.mu.Lock()
		if _, ok := s.docs[req.uri]; ok {
			s.mu.Unlock()
			continue
		}
		s.docs[req.uri] = 1
		s.mu.Unlock()
		_ = s.sendNotification("textDocument/didOpen", didOpenParams{
			TextDocument: textDocumentItem{
				URI:        req.uri,
				LanguageID: req.languageID,
				Version:    1,
				Text:       req.text,
			},
		})
	}
}

func (s *server) sendNotification(method string, params any) error {
	msg := rpcNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	return s.send(msg)
}

func (s *server) send(v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(payload))
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := io.WriteString(s.stdin, header); err != nil {
		return err
	}
	_, err = s.stdin.Write(payload)
	return err
}

// request sends a JSON-RPC request and waits for the response
func (s *server) request(method string, params any) (json.RawMessage, error) {
	s.mu.Lock()
	if !s.initialized {
		s.mu.Unlock()
		return nil, errors.New("LSP server not initialized")
	}
	s.nextID++
	id := s.nextID
	ch := make(chan json.RawMessage, 1)
	s.handlers[id] = ch
	s.mu.Unlock()

	msg := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	if err := s.send(msg); err != nil {
		s.mu.Lock()
		delete(s.handlers, id)
		s.mu.Unlock()
		return nil, err
	}

	// Wait for response with timeout
	select {
	case result := <-ch:
		return result, nil
	case <-time.After(10 * time.Second):
		s.mu.Lock()
		delete(s.handlers, id)
		s.mu.Unlock()
		return nil, errors.New("LSP request timeout")
	}
}

// LSPError represents an error returned by the LSP server
type LSPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// requestLocations sends a request and parses the response as Location or []Location
func (s *server) requestLocations(method string, params any) ([]Location, error) {
	result, err := s.request(method, params)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	if len(result) == 0 || string(result) == "null" {
		return nil, nil
	}

	// Check if this is an LSP error response
	var lspErr LSPError
	if err := json.Unmarshal(result, &lspErr); err == nil && lspErr.Message != "" {
		return nil, fmt.Errorf("LSP: %s", lspErr.Message)
	}

	// LSP can return Location, []Location, or LocationLink[]
	// Try to parse as []Location first
	var locs []Location
	if err := json.Unmarshal(result, &locs); err == nil {
		return locs, nil
	}

	// Try to parse as single Location
	var loc Location
	if err := json.Unmarshal(result, &loc); err == nil {
		if loc.URI != "" {
			return []Location{loc}, nil
		}
	}

	// Try to parse as LocationLink[]
	var links []struct {
		TargetURI            string `json:"targetUri"`
		TargetRange          Range  `json:"targetRange"`
		TargetSelectionRange Range  `json:"targetSelectionRange"`
	}
	if err := json.Unmarshal(result, &links); err == nil && len(links) > 0 {
		locs = make([]Location, len(links))
		for i, link := range links {
			locs[i] = Location{
				URI:   link.TargetURI,
				Range: link.TargetSelectionRange,
			}
		}
		return locs, nil
	}

	return nil, nil
}

func (s *server) sendEvent(kind, msg string) {
	select {
	case s.events <- Event{Kind: kind, Message: msg}:
	default:
	}
}

func (s *server) stop() {
	if s.cmd == nil || s.cmd.Process == nil {
		return
	}
	_ = s.cmd.Process.Kill()
	_, _ = s.cmd.Process.Wait()
}

func readMessage(r *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.ToLower(strings.TrimSpace(parts[0])) == "content-length" {
			val := strings.TrimSpace(parts[1])
			if n, err := strconv.Atoi(val); err == nil {
				length = n
			}
		}
	}
	if length < 0 {
		return nil, errors.New("missing content-length")
	}
	buf := make([]byte, length)
	_, err := io.ReadFull(r, buf)
	return buf, err
}

func findRoot(path string, markers []string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Dir(path)
	}
	dir := filepath.Dir(abs)
	if len(markers) == 0 {
		return dir
	}
	for {
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return filepath.Dir(abs)
}

func fileURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	return u.String()
}

// URIToPath converts a file:// URI to a filesystem path
func URIToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return uri
	}
	if u.Scheme != "file" {
		return uri
	}
	return filepath.FromSlash(u.Path)
}
