package diagnostic

import (
	"errors"
	"testing"

	"github.com/expr-lang/expr/file"
	"github.com/expr-lang/expr/internal/testify/assert"
	"github.com/expr-lang/expr/internal/testify/require"
)

func TestCollectorAggregatesSameCategoryAndLocation(t *testing.T) {
	c := NewCollector()
	loc := file.Location{From: 1, To: 3}

	c.Report(CategoryShadow, loc, "first")
	c.Report(CategoryShadow, loc, "second")
	c.Report(CategoryShadow, loc, "third")

	diags := c.Diagnostics()
	require.Len(t, diags, 1)
	assert.Equal(t, CategoryShadow, diags[0].Category)
	assert.Equal(t, loc, diags[0].Location)
	assert.Equal(t, "first", diags[0].Message)
	assert.Equal(t, 3, diags[0].Count)
}

func TestCollectorKeepsDifferentCategoriesAndLocations(t *testing.T) {
	c := NewCollector()
	c.Report(CategoryShadow, file.Location{From: 1, To: 2}, "a")
	c.Report(CategoryBuiltinShadow, file.Location{From: 1, To: 2}, "b")
	c.Report(CategoryShadow, file.Location{From: 3, To: 4}, "c")

	assert.Len(t, c.Diagnostics(), 3)
}

func TestCollectorSortsByPosition(t *testing.T) {
	c := NewCollector()
	c.Report(CategoryShadow, file.Location{From: 9, To: 10}, "late")
	c.Report(CategoryConstantFold, file.Location{From: 0, To: 3}, "early")
	c.Report(CategoryBuiltinShadow, file.Location{From: 5, To: 6}, "middle")

	diags := c.Diagnostics()
	require.Len(t, diags, 3)
	assert.Equal(t, 0, diags[0].Location.From)
	assert.Equal(t, 5, diags[1].Location.From)
	assert.Equal(t, 9, diags[2].Location.From)
}

func TestCollectorSuppress(t *testing.T) {
	c := NewCollector(Suppress(CategoryShadow))
	c.Report(CategoryShadow, file.Location{From: 1, To: 2}, "hidden")
	c.Report(CategoryBuiltinShadow, file.Location{From: 3, To: 4}, "visible")

	diags := c.Diagnostics()
	require.Len(t, diags, 1)
	assert.Equal(t, CategoryBuiltinShadow, diags[0].Category)

	// Suppressed diagnostics do not trigger strict mode.
	_, ok := c.FirstUpgradable()
	require.True(t, ok)

	c.Suppress(CategoryBuiltinShadow, CategoryConstantFold)
	_, ok = c.FirstUpgradable()
	assert.False(t, ok)
}

func TestFirstUpgradableIgnoresErrors(t *testing.T) {
	c := NewCollector()
	c.Report(CategoryParseError, file.Location{From: 0, To: 1}, "parse error")
	c.Report(CategoryTypeError, file.Location{From: 2, To: 3}, "type error")

	_, ok := c.FirstUpgradable()
	assert.False(t, ok)

	c.Report(CategoryConstantFold, file.Location{From: 4, To: 5}, "folded")
	d, ok := c.FirstUpgradable()
	require.True(t, ok)
	assert.Equal(t, CategoryConstantFold, d.Category)
}

func TestNilCollectorIsNoOp(t *testing.T) {
	var c *Collector
	c.Report(CategoryShadow, file.Location{From: 0, To: 1}, "ignored")
	c.ReportError(CategoryParseError, &file.Error{Message: "ignored"})
	assert.Nil(t, c.Diagnostics())
}

func TestDiagnosticBind(t *testing.T) {
	d := Diagnostic{
		Category: CategoryShadow,
		Location: file.Location{From: 4, To: 5},
		Message:  "loop variable # shadows loop variable in enclosing predicate",
		Count:    1,
	}
	bound := d.Bind(file.NewSource("a + b"))
	assert.Equal(t, 1, bound.Line)
	assert.Equal(t, 4, bound.Column)
	assert.Contains(t, bound.Error(), "loop variable # shadows loop variable in enclosing predicate (1:5)")
	assert.Contains(t, bound.Error(), "^")
}

func TestDiagnosticErrorAnnotatesCount(t *testing.T) {
	d := Diagnostic{
		Category: CategoryShadow,
		Location: file.Location{From: 0, To: 1},
		Message:  "shadow",
		Count:    3,
	}
	assert.Equal(t, "shadow (repeated 3 times)", d.Error())
}

func TestRuntimeErrorUnwrapsFileError(t *testing.T) {
	fe := &file.Error{Message: "runtime error: boom"}
	e := &Error{Category: CategoryRuntimeError, Err: fe}

	assert.Equal(t, "runtime error: boom", e.Error())
	assert.Equal(t, CategoryRuntimeError, e.Category)

	var target *file.Error
	require.True(t, errors.As(e, &target))
	assert.Equal(t, "runtime error: boom", target.Message)
}
