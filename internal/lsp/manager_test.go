package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kobzarvs/qedit/internal/config"
)

func TestReadMessage(t *testing.T) {
	msg := "Content-Length: 4\r\n\r\ntest"
	out, err := readMessage(bufio.NewReader(strings.NewReader(msg)))
	if err != nil {
		t.Fatalf("readMessage error: %v", err)
	}
	if string(out) != "test" {
		t.Fatalf("readMessage = %q, want %q", string(out), "test")
	}
}

func TestFindRoot(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "root")
	sub := filepath.Join(root, "sub", "inner")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	marker := filepath.Join(root, "go.mod")
	if err := os.WriteFile(marker, []byte("module test"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	path := filepath.Join(sub, "main.go")
	got := findRoot(path, []string{"go.mod"})
	if got != root {
		t.Fatalf("findRoot = %q, want %q", got, root)
	}
}

func TestFileURI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file with spaces.go")
	uri := fileURI(path)
	if !strings.HasPrefix(uri, "file://") {
		t.Fatalf("fileURI = %q, want file:// prefix", uri)
	}
}

func TestManagerOpenFileSendsDidOpen(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "lsp-events.txt")
	t.Setenv("QEDIT_LSP_HELPER", "1")
	t.Setenv("QEDIT_LSP_OUT", outPath)

	langs := config.Languages{
		Languages: []config.Language{
			{
				Name:            "go",
				FileTypes:       []string{"go"},
				LanguageServers: []string{"helper"},
			},
		},
		LanguageServers: map[string]config.LanguageServer{
			"helper": {
				Command: os.Args[0],
				Args:    []string{"-test.run=TestLSPServerHelper", "--"},
			},
		},
	}

	m := NewManager(langs)
	if err := m.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer func() { _ = m.Stop() }()

	path := filepath.Join(dir, "main.go")
	m.OpenFile(path, "package main\n")
	m.OpenFile(path, "package main\n")

	text := waitForHelperOutput(t, outPath, 2*time.Second)
	if !strings.Contains(text, "initialize") {
		t.Fatalf("missing initialize in %q", text)
	}
	if !strings.Contains(text, "textDocument/didOpen") {
		t.Fatalf("missing didOpen in %q", text)
	}
	if strings.Count(text, "textDocument/didOpen") != 1 {
		t.Fatalf("expected 1 didOpen, got %d", strings.Count(text, "textDocument/didOpen"))
	}
	if strings.Count(text, "initialized") != 1 {
		t.Fatalf("expected initialized notification, got %d", strings.Count(text, "initialized"))
	}
	lines := strings.Split(strings.TrimSpace(text), "\n")
	idxInitialize := indexOf(lines, "initialize")
	idxInitialized := indexOf(lines, "initialized")
	idxDidOpen := indexOf(lines, "textDocument/didOpen")
	if idxInitialize == -1 || idxInitialized == -1 || idxDidOpen == -1 {
		t.Fatalf("missing methods in %v", lines)
	}
	if !(idxInitialize < idxInitialized && idxInitialized < idxDidOpen) {
		t.Fatalf("order = %v, want initialize -> initialized -> didOpen", lines)
	}
}

func TestLSPServerHelper(t *testing.T) {
	if os.Getenv("QEDIT_LSP_HELPER") != "1" {
		return
	}
	outPath := os.Getenv("QEDIT_LSP_OUT")
	if outPath == "" {
		os.Exit(2)
	}
	time.AfterFunc(2*time.Second, func() {
		os.Exit(2)
	})

	reader := bufio.NewReader(os.Stdin)
	var methods []string
	for {
		msg, err := readMessage(reader)
		if err != nil {
			break
		}
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(msg, &envelope); err != nil {
			continue
		}
		rawMethod, ok := envelope["method"]
		if !ok {
			continue
		}
		var method string
		if err := json.Unmarshal(rawMethod, &method); err != nil {
			continue
		}
		methods = append(methods, method)
		if method == "initialize" {
			var id int
			_ = json.Unmarshal(envelope["id"], &id)
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      id,
				"result":  map[string]any{"capabilities": map[string]any{}},
			}
			payload, err := json.Marshal(resp)
			if err == nil {
				header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(payload))
				_, _ = os.Stdout.Write([]byte(header))
				_, _ = os.Stdout.Write(payload)
			}
		}
		if method == "textDocument/didOpen" && len(methods) >= 3 {
			break
		}
	}
	_ = os.WriteFile(outPath, []byte(strings.Join(methods, "\n")), 0o644)
}

func waitForHelperOutput(t *testing.T, path string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil {
			return string(b)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no helper output")
	return ""
}

func indexOf(lines []string, value string) int {
	for i, line := range lines {
		if line == value {
			return i
		}
	}
	return -1
}
