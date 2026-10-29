package docgen

import (
	"fmt"
	"strings"

	"github.com/expr-lang/expr/builtin"
)

// Functions catalog region markers in docs/language-definition.md.
const (
	FunctionsMarkdownBegin = "<!-- BEGIN BUILTIN FUNCTIONS -->"
	FunctionsMarkdownEnd   = "<!-- END BUILTIN FUNCTIONS -->"
)

// CategoryPreamble holds optional category-level documentation prose that is
// not specific to a single function (the Date category introduction).
type CategoryPreamble struct {
	Intro  string
	Footer string
}

// categoryPreambles returns category-level prose kept verbatim from the
// former hand-written documentation.
func categoryPreambles() map[builtin.Category]CategoryPreamble {
	return map[builtin.Category]CategoryPreamble{
		builtin.CategoryDate: {
			Intro: "Expr has a built-in support for Go's [time package](https://pkg.go.dev/time).\n" +
				"It is possible to subtract two dates and get the duration between them:\n\n" +
				"```expr\ncreatedAt - now()\n```\n\n" +
				"It is possible to add a duration to a date:\n\n" +
				"```expr\ncreatedAt + duration(\"1h\")\n```\n\n" +
				"And it is possible to compare dates:\n\n" +
				"```expr\ncreatedAt > now() - duration(\"1h\")\n```\n\n",
			Footer: "To convert a date to a different timezone, use the [`In()`](https://pkg.go.dev/time#Time.In) method:\n\n" +
				"```expr\ndate(\"2023-08-14 00:00:00\").In(timezone(\"Europe/Zurich\"))\n```\n",
		},
	}
}

// RenderFunctionsMarkdown renders the complete builtin functions catalog for
// docs/language-definition.md, derived from the builtin registry.
func RenderFunctionsMarkdown() string {
	preambles := categoryPreambles()

	byCategory := make(map[builtin.Category][]*builtin.Function)
	for _, fn := range builtin.Builtins {
		byCategory[fn.Category] = append(byCategory[fn.Category], fn)
	}

	var b strings.Builder
	for _, category := range builtin.CategoryOrder {
		fns := byCategory[category]
		if len(fns) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", category.Title())
		if p, ok := preambles[category]; ok && p.Intro != "" {
			b.WriteString(p.Intro)
		}
		for _, fn := range fns {
			renderFunctionMarkdown(&b, fn)
		}
		if p, ok := preambles[category]; ok && p.Footer != "" {
			b.WriteString(p.Footer)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func renderFunctionMarkdown(b *strings.Builder, fn *builtin.Function) {
	fmt.Fprintf(b, "### %s {#%s}\n\n", fn.Signature, fn.Name)
	b.WriteString(fn.Description)
	b.WriteString("\n\n```expr\n")
	b.WriteString(strings.Join(fn.Examples, "\n"))
	b.WriteString("\n```\n\n")
}

// ExtractFunctionsMarkdown returns the content between the region markers.
func ExtractFunctionsMarkdown(doc string) (string, bool) {
	begin := strings.Index(doc, FunctionsMarkdownBegin)
	if begin < 0 {
		return "", false
	}
	begin += len(FunctionsMarkdownBegin)
	end := strings.Index(doc[begin:], FunctionsMarkdownEnd)
	if end < 0 {
		return "", false
	}
	return doc[begin : begin+end], true
}

// ReplaceFunctionsMarkdown rewrites the marked region with the freshly
// rendered catalog, preserving everything outside the markers.
func ReplaceFunctionsMarkdown(doc, region string) string {
	begin := strings.Index(doc, FunctionsMarkdownBegin)
	end := strings.Index(doc, FunctionsMarkdownEnd)
	if begin < 0 || end < 0 || end < begin {
		return doc
	}
	return doc[:begin] + FunctionsMarkdownBegin + region + doc[end:]
}
