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
		name:      name,
		cmd:       cmd,
		stdin:     stdin,
		reader:    bufio.NewReader(stdout),
		rootURI:   rootURI,
		events:    m.events,
		docs:      make(map[string]int),
		initID:    -1,
		nextID:    0,
		initialized: false,
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
}

type openRequest struct {
	uri        string
	languageID string
	text       string
}

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type rpcNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
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
				s.handleResponse(id)
			}
			continue
		}
	}
}

func (s *server) handleResponse(id int) {
	s.mu.Lock()
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

func (s *server) sendNotification(method string, params interface{}) error {
	msg := rpcNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	return s.send(msg)
}

func (s *server) send(v interface{}) error {
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
