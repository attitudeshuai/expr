package docgen_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/expr-lang/expr/builtin"
	. "github.com/expr-lang/expr/docgen"
	"github.com/expr-lang/expr/internal/testify/assert"
	"github.com/expr-lang/expr/internal/testify/require"
)

func countPrefix(md, prefix string) int {
	var n int
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, prefix) {
			n++
		}
	}
	return n
}

func TestCreateDoc_BuiltinCatalogMarkdown(t *testing.T) {
	doc := CreateDoc(map[string]any{})
	require.Len(t, doc.BuiltinFunctions, len(builtin.CategoryOrder))

	md := doc.Markdown()
	assert.Contains(t, md, "### Builtin Functions")

	for _, category := range builtin.CategoryOrder {
		assert.Contains(t, md, "#### "+category.Title())
	}

	// One signature heading per registered builtin.
	assert.Equal(t, len(builtin.Builtins), countPrefix(md, "##### "))
	assert.Contains(t, md, "##### trim(str[, chars])")
	assert.Contains(t, md, "##### get(v, index)")
	assert.Contains(t, md, "##### date(str[, format[, timezone]])")

	// Descriptions and examples are rendered.
	assert.Contains(t, md, "Removes white spaces from both ends")
	assert.Contains(t, md, "trim(\"  Hello  \") == \"Hello\"")
}

func TestCreateDoc_BuiltinCatalogJSON(t *testing.T) {
	doc := CreateDoc(map[string]any{})
	b, err := json.Marshal(doc)
	require.NoError(t, err)
	s := string(b)
	assert.Contains(t, s, `"builtin_functions"`)
	assert.Contains(t, s, `"signature":"trim(str[, chars])"`)
	assert.Contains(t, s, `"name":"bitand"`)
}

func TestCreateDoc_BuiltinsSuppressed(t *testing.T) {
	saved := Builtins
	Builtins = nil
	t.Cleanup(func() { Builtins = saved })

	doc := CreateDoc(map[string]any{})
	assert.Nil(t, doc.BuiltinFunctions)
	assert.NotContains(t, doc.Markdown(), "### Builtin Functions")
}
