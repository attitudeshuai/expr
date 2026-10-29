package docgen_test

import (
	"os"
	"strings"
	"testing"

	"github.com/expr-lang/expr/builtin"
	. "github.com/expr-lang/expr/docgen"
	"github.com/expr-lang/expr/internal/testify/assert"
	"github.com/expr-lang/expr/internal/testify/require"
)

const docsPath = "../docs/language-definition.md"

func TestFunctionsCatalogUpToDate(t *testing.T) {
	b, err := os.ReadFile(docsPath)
	require.NoError(t, err)
	doc := string(b)

	region, ok := ExtractFunctionsMarkdown(doc)
	require.True(t, ok, "catalog markers missing from %s", docsPath)

	expected := "\n\n" + RenderFunctionsMarkdown()
	require.Equal(t, expected, region,
		"docs/language-definition.md is stale; run `go run ./docgen/gen -w docs/language-definition.md`")
}

func TestFunctionsCatalogComplete(t *testing.T) {
	rendered := RenderFunctionsMarkdown()

	// Every category heading present, in canonical order.
	pos := -1
	for _, category := range builtin.CategoryOrder {
		heading := "## " + category.Title()
		at := strings.Index(rendered, heading)
		require.True(t, at > pos, "category %q missing or out of order", heading)
		pos = at
	}

	// Every registered function present with verbatim signature heading,
	// anchor, description and an example fence.
	for _, fn := range builtin.Builtins {
		heading := "### " + fn.Signature + " {#" + fn.Name + "}"
		assert.True(t, strings.Contains(rendered, heading), "missing signature heading %q", heading)
		assert.True(t, strings.Contains(rendered, fn.Description), "missing description for %q", fn.Name)
		for _, example := range fn.Examples {
			assert.True(t, strings.Contains(rendered, example), "missing example for %q: %s", fn.Name, example)
		}
	}
}

func TestReplaceFunctionsMarkdownIdempotent(t *testing.T) {
	b, err := os.ReadFile(docsPath)
	require.NoError(t, err)

	region := "\n\n" + RenderFunctionsMarkdown()
	once := ReplaceFunctionsMarkdown(string(b), region)
	twice := ReplaceFunctionsMarkdown(once, region)
	assert.Equal(t, once, twice)
}
