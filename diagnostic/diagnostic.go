// Package diagnostic provides a unified diagnostic pipeline shared by the
// parser, the type checker, the optimizer and the virtual machine.
//
// Every signal produced during compilation or evaluation is reported as a
// Diagnostic carrying a Category and a source Location. Signals with the same
// category and location are aggregated into a single diagnostic annotated with
// the number of occurrences. Categories can be suppressed, and the warning
// categories can be upgraded to compilation failures in strict mode.
package diagnostic

import (
	"fmt"
	"sort"
	"strings"

	"github.com/expr-lang/expr/file"
)

// Category classifies a diagnostic by its origin.
type Category string

const (
	// CategoryParseError is reported for parser and lexer errors.
	CategoryParseError Category = "parse-error"

	// CategoryTypeError is reported for type checking errors.
	CategoryTypeError Category = "type-error"

	// CategoryOptimizationError is reported for errors produced while
	// optimizing (folding) the expression.
	CategoryOptimizationError Category = "optimization-error"

	// CategoryRuntimeError is reported for value retrieval and evaluation
	// failures at runtime.
	CategoryRuntimeError Category = "runtime-error"

	// CategoryShadow is reported when a loop variable of a nested predicate
	// shadows a loop variable of an enclosing predicate.
	CategoryShadow Category = "shadow"

	// CategoryBuiltinShadow is reported when a builtin function silently
	// takes precedence over an environment field with the same name.
	CategoryBuiltinShadow Category = "builtin-shadow"

	// CategoryConstantFold is reported when the whole expression is folded
	// to a constant during optimization.
	CategoryConstantFold Category = "constant-fold"
)

// upgradableCategories are warning categories which do not fail compilation by
// themselves, but which can be upgraded to failures in strict mode.
var upgradableCategories = map[Category]bool{
	CategoryShadow:        true,
	CategoryBuiltinShadow: true,
	CategoryConstantFold:  true,
}

// Upgradable reports whether diagnostics of this category can be upgraded to
// compilation failures in strict mode.
func (c Category) Upgradable() bool {
	return upgradableCategories[c]
}

// Diagnostic is a single signal produced during compilation or evaluation.
// Signals sharing the same Category and Location are aggregated, therefore
// Count carries the number of aggregated occurrences.
type Diagnostic struct {
	Category Category      `json:"category"`
	Location file.Location `json:"location"`
	Line     int           `json:"line"`
	Column   int           `json:"column"`
	Message  string        `json:"message"`
	Snippet  string        `json:"snippet"`
	Count    int           `json:"count"`
}

// Bind resolves byte offsets of the location to line, column and a source
// snippet. It mirrors file.Error.Bind so that diagnostics and errors render
// identically.
func (d Diagnostic) Bind(source file.Source) Diagnostic {
	src := source.String()

	var runeCount, lineStart int
	d.Line = 1
	d.Column = 0
	for i, r := range src {
		if runeCount == d.Location.From {
			break
		}
		if r == '\n' {
			lineStart = i + 1
			d.Line++
			d.Column = 0
		} else {
			d.Column++
		}
		runeCount++
	}

	lineEnd := lineStart + strings.IndexByte(src[lineStart:], '\n')
	if lineEnd < lineStart {
		lineEnd = len(src)
	}
	if lineStart == lineEnd {
		return d
	}

	const prefix = "\n | "
	line := src[lineStart:lineEnd]
	snippet := new(strings.Builder)
	snippet.Grow(2*len(prefix) + len(line) + d.Column + 1)
	snippet.WriteString(prefix)
	tabReplacer.WriteString(snippet, line)
	snippet.WriteString(prefix)
	for i := 0; i < d.Column; i++ {
		snippet.WriteByte('.')
	}
	snippet.WriteByte('^')
	d.Snippet = snippet.String()
	return d
}

var tabReplacer = strings.NewReplacer("\t", " ")

// message returns the message annotated with the aggregation count when the
// diagnostic represents more than one occurrence.
func (d Diagnostic) message() string {
	if d.Count > 1 {
		return fmt.Sprintf("%s (repeated %d times)", d.Message, d.Count)
	}
	return d.Message
}

// Error allows an upgraded (strict mode) diagnostic to be returned as an
// error. The rendering is compatible with file.Error.
func (d Diagnostic) Error() string {
	if d.Snippet == "" {
		return d.message()
	}
	return fmt.Sprintf(
		"%s (%d:%d)%s",
		d.message(),
		d.Line,
		d.Column+1, // add one to the 0-based column for display
		d.Snippet,
	)
}

// Error wraps a bound file.Error with a category, so that runtime failures
// keep their original text and location verbatim while becoming part of the
// diagnostic pipeline.
type Error struct {
	Category Category `json:"category"`
	Err      *file.Error
}

// Error returns the original file.Error text verbatim.
func (e *Error) Error() string {
	return e.Err.Error()
}

// Unwrap exposes the wrapped file.Error to errors.Is and errors.As.
func (e *Error) Unwrap() error {
	return e.Err
}

// Option configures a Collector.
type Option func(*Collector)

// Suppress returns an option which suppresses diagnostics of the given
// categories. Suppressed diagnostics are neither delivered nor able to fail
// the compilation in strict mode. Error categories still fail compilation,
// regardless of suppression.
func Suppress(categories ...Category) Option {
	return func(c *Collector) {
		c.Suppress(categories...)
	}
}

// NewCollector creates a diagnostic Collector.
func NewCollector(options ...Option) *Collector {
	c := &Collector{
		suppressed: make(map[Category]bool),
	}
	for _, option := range options {
		option(c)
	}
	return c
}

// Collector aggregates diagnostics reported by the compilation stages and by
// the runtime. The zero value is not ready for use, create one with
// NewCollector.
type Collector struct {
	suppressed map[Category]bool
	entries    []Diagnostic
}

// Suppress suppresses diagnostics of the given categories.
func (c *Collector) Suppress(categories ...Category) {
	if c.suppressed == nil {
		c.suppressed = make(map[Category]bool)
	}
	for _, category := range categories {
		c.suppressed[category] = true
	}
}

// Report records a diagnostic with the given category, location and message.
// Reports with the same category and location are aggregated.
func (c *Collector) Report(category Category, location file.Location, message string) {
	if c == nil {
		return
	}
	for i := range c.entries {
		if c.entries[i].Category == category && c.entries[i].Location == location {
			c.entries[i].Count++
			return
		}
	}
	c.entries = append(c.entries, Diagnostic{
		Category: category,
		Location: location,
		Message:  message,
		Count:    1,
	})
}

// ReportError records an existing file.Error under the given category, keeping
// its location and message verbatim.
func (c *Collector) ReportError(category Category, err *file.Error) {
	if c == nil || err == nil {
		return
	}
	c.Report(category, err.Location, err.Message)
}

// Diagnostics returns collected diagnostics sorted by position (and by first
// occurrence for ties), with same-category same-location signals aggregated.
// Suppressed categories are omitted.
func (c *Collector) Diagnostics() []Diagnostic {
	if c == nil {
		return nil
	}
	result := make([]Diagnostic, 0, len(c.entries))
	for _, d := range c.entries {
		if c.suppressed[d.Category] {
			continue
		}
		result = append(result, d)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Location.From != result[j].Location.From {
			return result[i].Location.From < result[j].Location.From
		}
		if result[i].Location.To != result[j].Location.To {
			return result[i].Location.To < result[j].Location.To
		}
		return result[i].Category < result[j].Category
	})
	return result
}

// FirstUpgradable returns the first (by position) reported warning diagnostic
// which can be upgraded to a compilation failure in strict mode. Suppressed
// categories are not considered.
func (c *Collector) FirstUpgradable() (Diagnostic, bool) {
	var (
		found    Diagnostic
		hasFound bool
	)
	for _, d := range c.Diagnostics() {
		if !d.Category.Upgradable() {
			continue
		}
		if !hasFound || d.Location.From < found.Location.From {
			found = d
			hasFound = true
		}
	}
	return found, hasFound
}
