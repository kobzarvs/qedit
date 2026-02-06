package treesitter

import (
	"context"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	tree_sitter_vue "github.com/kobzarvs/qedit/internal/treesitter/vue"
)

func TestVueHighlightQueryCompiles(t *testing.T) {
	lang := tree_sitter_vue.GetLanguage()
	query, err := sitter.NewQuery([]byte(vueHighlightQuery), lang)
	if err != nil {
		t.Fatalf("vueHighlightQuery failed to compile: %v", err)
	}
	if query == nil {
		t.Fatal("query is nil despite no error")
	}
}

func TestVueHighlights(t *testing.T) {
	lang := tree_sitter_vue.GetLanguage()

	code := `<template>
  <div class="app" v-if="show" @click="handler" :title="msg">
    <span>{{ message }}</span>
    <!-- a comment -->
  </div>
</template>

<script>
export default {
  data() {
    return { message: 'hello' }
  }
}
</script>

<style scoped>
.app { color: red; }
</style>
`

	parser := sitter.NewParser()
	parser.SetLanguage(lang)
	tree, err := parser.ParseCtx(context.Background(), nil, []byte(code))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	query, err := sitter.NewQuery([]byte(vueHighlightQuery), lang)
	if err != nil {
		t.Fatalf("Query compile error: %v", err)
	}

	qc := sitter.NewQueryCursor()
	qc.Exec(query, tree.RootNode())

	captures := make(map[string][]string)
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

	// Tag names
	assertContains(t, captures["type"], "template")
	assertContains(t, captures["type"], "div")
	assertContains(t, captures["type"], "span")
	assertContains(t, captures["type"], "script")
	assertContains(t, captures["type"], "style")

	// Attributes
	assertContains(t, captures["field"], "class")
	assertContains(t, captures["field"], "scoped")

	// Quoted attribute values
	assertContains(t, captures["string"], `"app"`)

	// Directives
	assertContains(t, captures["field"], "v-if")

	// Directive values: @click → "click" captured as function
	assertContains(t, captures["function"], "click")

	// Directive values: :title → "title" captured as variable
	assertContains(t, captures["variable"], "title")

	// Interpolation text
	assertContains(t, captures["variable"], " message ")

	// Comment
	assertContains(t, captures["comment"], "<!-- a comment -->")

	// Script raw text
	if len(captures["string"]) == 0 {
		t.Error("expected string captures for script/style raw text")
	}
}
