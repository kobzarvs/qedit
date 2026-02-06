package treesitter

import (
	"context"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
)

func TestJavaScriptHighlightQueryCompiles(t *testing.T) {
	lang := javascript.GetLanguage()
	query, err := sitter.NewQuery([]byte(javascriptHighlightQuery), lang)
	if err != nil {
		t.Fatalf("javascriptHighlightQuery failed to compile: %v", err)
	}
	if query == nil {
		t.Fatal("query is nil despite no error")
	}
}

func TestJavaScriptHighlights(t *testing.T) {
	lang := javascript.GetLanguage()

	code := `const greeting = "hello world";
function add(a, b) {
  return a + b;
}
class MyClass extends Base {
  constructor() {
    super();
    this.value = 42;
  }
}
const obj = { name: "test", value: undefined };
`

	parser := sitter.NewParser()
	parser.SetLanguage(lang)
	tree, err := parser.ParseCtx(context.Background(), nil, []byte(code))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	query, err := sitter.NewQuery([]byte(javascriptHighlightQuery), lang)
	if err != nil {
		t.Fatalf("Query compile error: %v", err)
	}

	qc := sitter.NewQueryCursor()
	qc.Exec(query, tree.RootNode())

	captures := make(map[string][]string) // kind -> list of matched text
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, c := range m.Captures {
			name := query.CaptureNameForId(c.Index)
			text := c.Node.Content([]byte(code))
			captures[name] = append(captures[name], text)
		}
	}

	// Verify keywords
	assertContains(t, captures["keyword"], "const")
	assertContains(t, captures["keyword"], "function")
	assertContains(t, captures["keyword"], "return")
	assertContains(t, captures["keyword"], "class")
	assertContains(t, captures["keyword"], "extends")
	assertContains(t, captures["keyword"], "super")
	assertContains(t, captures["keyword"], "this")

	// Verify strings
	assertContains(t, captures["string"], `"hello world"`)
	assertContains(t, captures["string"], `"test"`)

	// Verify numbers
	assertContains(t, captures["number"], "42")

	// Verify function names
	assertContains(t, captures["function"], "add")
	assertContains(t, captures["function"], "constructor")

	// Verify type (class name)
	assertContains(t, captures["type"], "MyClass")

	// Verify fields (property access)
	assertContains(t, captures["field"], "value")
	assertContains(t, captures["field"], "name")

	// Verify constants
	assertContains(t, captures["constant"], "undefined")

	// Verify parameters
	assertContains(t, captures["parameter"], "a")
	assertContains(t, captures["parameter"], "b")
}

func assertContains(t *testing.T, items []string, want string) {
	t.Helper()
	for _, item := range items {
		if item == want {
			return
		}
	}
	t.Errorf("expected capture list to contain %q, got %v", want, items)
}
