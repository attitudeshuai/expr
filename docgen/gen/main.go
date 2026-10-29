// Command gen regenerates the builtin functions catalog in
// docs/language-definition.md from the builtin registry.
//
// Without flags it prints the catalog region to stdout:
//
//	go run ./docgen/gen
//
// To rewrite the marked region in the documentation file:
//
//	go run ./docgen/gen -w docs/language-definition.md
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/expr-lang/expr/docgen"
)

func main() {
	write := flag.String("w", "", "documentation file to rewrite in place")
	flag.Parse()

	region := "\n\n" + docgen.RenderFunctionsMarkdown()

	if *write == "" {
		fmt.Print(region)
		return
	}

	b, err := os.ReadFile(*write)
	if err != nil {
		panic(err)
	}
	updated := docgen.ReplaceFunctionsMarkdown(string(b), region)
	if updated == string(b) {
		fmt.Fprintln(os.Stderr, "gen: function catalog markers not found")
		os.Exit(1)
	}
	if err := os.WriteFile(*write, []byte(updated), 0o644); err != nil {
		panic(err)
	}
}
