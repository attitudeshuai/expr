package expr_test

import (
	"errors"
	"testing"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/diagnostic"
	"github.com/expr-lang/expr/file"
	"github.com/expr-lang/expr/internal/testify/assert"
	"github.com/expr-lang/expr/internal/testify/require"
	"github.com/expr-lang/expr/types"
)

func categories(diags []diagnostic.Diagnostic) []diagnostic.Category {
	out := make([]diagnostic.Category, len(diags))
	for i, d := range diags {
		out[i] = d.Category
	}
	return out
}

func hasCategory(diags []diagnostic.Diagnostic, category diagnostic.Category) bool {
	for _, d := range diags {
		if d.Category == category {
			return true
		}
	}
	return false
}

func TestDiagnostics_legalExpressionsProduceNoDiagnostics(t *testing.T) {
	env := map[string]any{"foo": 1, "bar": 2}
	for _, code := range []string{
		"foo + bar",
		"foo > 0",
		"foo > 0 && bar < 100",
		"foo + 1",
	} {
		collector := diagnostic.NewCollector()
		_, err := expr.Compile(code, expr.Env(env), expr.WithDiagnostics(collector))
		require.NoError(t, err, code)
		assert.Empty(t, collector.Diagnostics(), code)
	}

	typedEnv := types.Map{
		"items": types.Array(types.Map{"a": types.Int, "b": types.Int}),
	}
	for _, code := range []string{
		`filter(items, .a > 0)`,
		`all(items, .a > 0)`,
		`map(items, .a + 1)`,
		`len("constant")`,
	} {
		collector := diagnostic.NewCollector()
		_, err := expr.Compile(code, expr.Env(typedEnv), expr.WithDiagnostics(collector))
		require.NoError(t, err, code)
		assert.Empty(t, collector.Diagnostics(), code)
	}
}

func TestDiagnostics_nestedPredicateShadow(t *testing.T) {
	env := types.Map{
		"items": types.Array(types.Map{"a": types.Int, "b": types.Int}),
	}
	code := `filter(filter(items, .a > 0), .b > 0)`

	collector := diagnostic.NewCollector()
	program, err := expr.Compile(code, expr.Env(env), expr.WithDiagnostics(collector))
	require.NoError(t, err)

	diags := collector.Diagnostics()
	require.Len(t, diags, 1)
	assert.Equal(t, diagnostic.CategoryShadow, diags[0].Category)
	assert.Equal(t, "loop variable # shadows loop variable in enclosing predicate", diags[0].Message)
	// The diagnostic is located at the inner filter(...) call.
	assert.Equal(t, 7, diags[0].Location.From)
	assert.Equal(t, 1, diags[0].Count)

	// The compilation output is unaffected and still runs.
	output, err := expr.Run(program, map[string]any{
		"items": []map[string]any{{"a": 1, "b": 1}, {"a": 0, "b": 0}},
	})
	require.NoError(t, err)
	assert.Equal(t, []any{map[string]any{"a": 1, "b": 1}}, output)
}

func TestDiagnostics_nestedPredicateShadowAggregatesRepeats(t *testing.T) {
	env := types.Map{
		"items": types.Array(types.Map{"a": types.Int}),
	}
	// The innermost predicate shadows two enclosing loop variables, both
	// reported at the same location and aggregated with the occurrence count.
	code := `filter(filter(filter(items, .a > 0), .a > 0), .a > 0)`

	collector := diagnostic.NewCollector()
	_, err := expr.Compile(code, expr.Env(env), expr.WithDiagnostics(collector))
	require.NoError(t, err)

	diags := collector.Diagnostics()
	// The middle predicate shadows the outermost one, and the innermost
	// predicate shadows both outer predicates (aggregated with count 2).
	require.Len(t, diags, 2)
	assert.Equal(t, diagnostic.CategoryShadow, diags[0].Category)
	assert.Equal(t, 7, diags[0].Location.From)
	assert.Equal(t, 1, diags[0].Count)
	assert.Equal(t, diagnostic.CategoryShadow, diags[1].Category)
	assert.Equal(t, 14, diags[1].Location.From)
	assert.Equal(t, 2, diags[1].Count)
}

func TestDiagnostics_nestedMapNamedLoopVariables(t *testing.T) {
	env := types.Map{
		"items": types.Array(types.Int),
	}
	code := `map(map(items, #), #)`

	collector := diagnostic.NewCollector()
	_, err := expr.Compile(code, expr.Env(env), expr.WithDiagnostics(collector))
	require.NoError(t, err)

	diags := collector.Diagnostics()
	require.Len(t, diags, 1)
	assert.Equal(t, diagnostic.CategoryShadow, diags[0].Category)
	// "#" and "#index" of the inner predicate both collide at one location.
	assert.Equal(t, 2, diags[0].Count)
}

func TestDiagnostics_builtinShadowsEnvironmentField(t *testing.T) {
	// Non-strict map: keys not statically declared may exist at runtime, yet
	// the builtin silently takes precedence.
	env := types.Map{
		"foo": types.String,
		types.Extra: types.Any,
	}

	collector := diagnostic.NewCollector()
	_, err := expr.Compile(`len(foo)`, expr.Env(env), expr.WithDiagnostics(collector))
	require.NoError(t, err)

	diags := collector.Diagnostics()
	require.Len(t, diags, 1)
	assert.Equal(t, diagnostic.CategoryBuiltinShadow, diags[0].Category)
	assert.Equal(t, 0, diags[0].Location.From)
	assert.Contains(t, diags[0].Message, `"len"`)
}

func TestDiagnostics_builtinShadowSortedByPosition(t *testing.T) {
	env := types.Map{types.Extra: types.Any}
	code := `len("a") + int(1)`

	collector := diagnostic.NewCollector()
	_, err := expr.Compile(code, expr.Env(env), expr.WithDiagnostics(collector))
	require.NoError(t, err)

	diags := collector.Diagnostics()
	require.Len(t, diags, 2)
	assert.Equal(t, diagnostic.CategoryBuiltinShadow, diags[0].Category)
	assert.Equal(t, diagnostic.CategoryBuiltinShadow, diags[1].Category)
	assert.True(t, diags[0].Location.From < diags[1].Location.From)
}

func TestDiagnostics_explicitBuiltinCallNotReported(t *testing.T) {
	env := types.Map{types.Extra: types.Any}

	collector := diagnostic.NewCollector()
	_, err := expr.Compile(`::len("a")`, expr.Env(env), expr.WithDiagnostics(collector))
	require.NoError(t, err)
	assert.Empty(t, collector.Diagnostics())
}

func TestDiagnostics_noEnvMeansNoBuiltinShadow(t *testing.T) {
	collector := diagnostic.NewCollector()
	_, err := expr.Compile(`len("a") + int(1)`, expr.WithDiagnostics(collector))
	require.NoError(t, err)
	assert.Empty(t, collector.Diagnostics())
}

func TestDiagnostics_constantFold(t *testing.T) {
	collector := diagnostic.NewCollector()
	_, err := expr.Compile(`1 + 2`, expr.WithDiagnostics(collector))
	require.NoError(t, err)
	diags := collector.Diagnostics()
	require.Len(t, diags, 1)
	assert.Equal(t, diagnostic.CategoryConstantFold, diags[0].Category)
	// The binary node location points at the operator.
	assert.Equal(t, 2, diags[0].Location.From)

	// A constant sub-expression which does not collapse the whole expression
	// is not reported.
	collector = diagnostic.NewCollector()
	_, err = expr.Compile(`foo + (1 + 2)`, expr.Env(map[string]any{"foo": 1}), expr.WithDiagnostics(collector))
	require.NoError(t, err)
	assert.Empty(t, collector.Diagnostics())

	// Cascading folds produce a single whole-expression diagnostic.
	collector = diagnostic.NewCollector()
	_, err = expr.Compile(`-(1 + 2)`, expr.WithDiagnostics(collector))
	require.NoError(t, err)
	diags = collector.Diagnostics()
	require.Len(t, diags, 1)
	assert.Equal(t, diagnostic.CategoryConstantFold, diags[0].Category)
	assert.Equal(t, 0, diags[0].Location.From)

	// A constant array is folded into a single constant.
	collector = diagnostic.NewCollector()
	_, err = expr.Compile(`[1, 2, 3]`, expr.WithDiagnostics(collector))
	require.NoError(t, err)
	diags = collector.Diagnostics()
	require.Len(t, diags, 1)
	assert.Equal(t, diagnostic.CategoryConstantFold, diags[0].Category)

	// Boolean short-circuit folding.
	collector = diagnostic.NewCollector()
	program, err := expr.Compile(`true or foo`, expr.Env(map[string]any{"foo": true}), expr.WithDiagnostics(collector))
	require.NoError(t, err)
	diags = collector.Diagnostics()
	require.Len(t, diags, 1)
	assert.Equal(t, diagnostic.CategoryConstantFold, diags[0].Category)
	output, err := expr.Run(program, map[string]any{"foo": false})
	require.NoError(t, err)
	assert.Equal(t, true, output)
}

func TestDiagnostics_constantFoldDisabledWithOptimizationOff(t *testing.T) {
	collector := diagnostic.NewCollector()
	_, err := expr.Compile(`1 + 2`, expr.Optimize(false), expr.WithDiagnostics(collector))
	require.NoError(t, err)
	assert.Empty(t, collector.Diagnostics())
}

func TestDiagnostics_suppressCategory(t *testing.T) {
	env := types.Map{
		"items": types.Array(types.Map{"a": types.Int}),
	}
	code := `filter(filter(items, .a > 0), .a > 0)`

	collector := diagnostic.NewCollector(diagnostic.Suppress(diagnostic.CategoryShadow))
	program, err := expr.Compile(code, expr.Env(env), expr.WithDiagnostics(collector))
	require.NoError(t, err)
	assert.Empty(t, collector.Diagnostics())

	output, err := expr.Run(program, map[string]any{"items": []map[string]any{{"a": 1}}})
	require.NoError(t, err)
	assert.Equal(t, []any{map[string]any{"a": 1}}, output)

	// A suppressed warning also does not fail strict mode.
	collector = diagnostic.NewCollector(diagnostic.Suppress(diagnostic.CategoryConstantFold))
	_, err = expr.Compile(`1 + 2`, expr.WithDiagnostics(collector), expr.StrictDiagnostics())
	require.NoError(t, err)
}

func TestDiagnostics_strictModeUpgradesFirstWarning(t *testing.T) {
	env := types.Map{
		"items": types.Array(types.Map{"a": types.Int}),
	}
	collector := diagnostic.NewCollector()
	_, err := expr.Compile(
		`filter(filter(items, .a > 0), .a > 0)`,
		expr.Env(env),
		expr.WithDiagnostics(collector),
		expr.StrictDiagnostics(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loop variable # shadows loop variable in enclosing predicate")
	assert.Contains(t, err.Error(), "(1:8)")
	// Diagnostics remain retrievable from the failed compilation.
	assert.NotEmpty(t, collector.Diagnostics())

	collector = diagnostic.NewCollector()
	_, err = expr.Compile(`1 + 2`, expr.WithDiagnostics(collector), expr.StrictDiagnostics())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expression folded to a constant")

	// Without strict mode the same inputs compile successfully.
	collector = diagnostic.NewCollector()
	_, err = expr.Compile(`1 + 2`, expr.WithDiagnostics(collector))
	require.NoError(t, err)
}

func TestDiagnostics_hardErrorsTakePrecedenceInStrictMode(t *testing.T) {
	collector := diagnostic.NewCollector()
	_, err := expr.Compile(`1 + true`, expr.WithDiagnostics(collector), expr.StrictDiagnostics())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid operation: + (mismatched types int and bool)")
	assert.True(t, hasCategory(collector.Diagnostics(), diagnostic.CategoryTypeError))
}

func TestDiagnostics_errorAndWarningsCollectedTogether(t *testing.T) {
	env := types.Map{
		"items": types.Array(types.Map{"a": types.Int, "b": types.Int}),
	}
	// The inner predicate returns an int (hard type error), while entering
	// the nested predicate also produces a shadow warning.
	code := `filter(filter(items, .a), .b > 0)`

	collector := diagnostic.NewCollector()
	_, err := expr.Compile(code, expr.Env(env), expr.WithDiagnostics(collector))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "predicate should return boolean")

	diags := collector.Diagnostics()
	assert.True(t, hasCategory(diags, diagnostic.CategoryTypeError))
	assert.True(t, hasCategory(diags, diagnostic.CategoryShadow))
	// Diagnostics are sorted by position: the shadow at the inner filter
	// comes before the error inside its predicate.
	assert.Equal(t, diagnostic.CategoryShadow, diags[0].Category)
}

func TestDiagnostics_parseErrorRoutedThroughPipeline(t *testing.T) {
	collector := diagnostic.NewCollector()
	_, err := expr.Compile(`1 +`, expr.WithDiagnostics(collector))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected token EOF")

	diags := collector.Diagnostics()
	require.Len(t, diags, 1)
	assert.Equal(t, diagnostic.CategoryParseError, diags[0].Category)
	assert.Equal(t, "unexpected token EOF", diags[0].Message)
}

func TestDiagnostics_patcherPassesDoNotMultiplyWarnings(t *testing.T) {
	env := types.Map{
		"items": types.Array(types.Map{"a": types.Int}),
	}

	collector := diagnostic.NewCollector()
	_, err := expr.Compile(
		`filter(filter(items, .a > 0), .a > 0)`,
		expr.Env(env),
		expr.Patch(noopVisitor{}),
		expr.WithDiagnostics(collector),
	)
	require.NoError(t, err)
	diags := collector.Diagnostics()
	require.Len(t, diags, 1)
	assert.Equal(t, 1, diags[0].Count)
}

type noopVisitor struct{}

func (noopVisitor) Visit(_ *ast.Node) {}

func TestDiagnostics_runtimeFailureIsCategorized(t *testing.T) {
	collector := diagnostic.NewCollector()
	program, err := expr.Compile(`foo.bar`, expr.AllowUndefinedVariables(), expr.WithDiagnostics(collector))
	require.NoError(t, err)

	_, err = expr.Run(program, nil)
	require.Error(t, err)

	diagnosticError, ok := err.(*diagnostic.Error)
	require.True(t, ok, "runtime error should be a *diagnostic.Error")
	assert.Equal(t, diagnostic.CategoryRuntimeError, diagnosticError.Category)
	// The original error text and location are preserved verbatim.
	assert.Equal(t, "cannot fetch foo from <nil> (1:1)\n | foo.bar\n | ^", diagnosticError.Error())

	var fileError *file.Error
	require.True(t, errors.As(err, &fileError))
	assert.Equal(t, "cannot fetch foo from <nil>", fileError.Message)

	// A non-foldable runtime arithmetic failure is categorized as well.
	collector = diagnostic.NewCollector()
	program, err = expr.Compile(`foo % 0`, expr.Env(map[string]any{"foo": 1}), expr.WithDiagnostics(collector))
	require.NoError(t, err)
	_, err = expr.Run(program, map[string]any{"foo": 1})
	require.Error(t, err)
	diagnosticError, ok = err.(*diagnostic.Error)
	require.True(t, ok)
	assert.Equal(t, diagnostic.CategoryRuntimeError, diagnosticError.Category)
	assert.Contains(t, diagnosticError.Error(), "integer divide by zero")
}

func TestDiagnostics_runtimeFailureUnchangedWithoutPipeline(t *testing.T) {
	program, err := expr.Compile(`foo.bar`, expr.AllowUndefinedVariables())
	require.NoError(t, err)

	_, err = expr.Run(program, nil)
	require.Error(t, err)
	fileError, ok := err.(*file.Error)
	require.True(t, ok, "runtime error should stay *file.Error without diagnostics enabled")
	assert.Equal(t, "cannot fetch foo from <nil> (1:1)\n | foo.bar\n | ^", fileError.Error())
}

func TestDiagnostics_defaultCompileIsIdenticalWithoutOption(t *testing.T) {
	// No WithDiagnostics: the program still compiles and runs, error text
	// and positions are unchanged (covered by the existing assertions as
	// well), no collector is involved.
	env := map[string]any{"foo": 1}
	program, err := expr.Compile(`foo + 1`, expr.Env(env))
	require.NoError(t, err)
	output, err := expr.Run(program, env)
	require.NoError(t, err)
	assert.Equal(t, 2, output)

	_, err = expr.Compile(`1 == true`)
	require.Error(t, err)
	fileError, ok := err.(*file.Error)
	require.True(t, ok)
	assert.Equal(t, "invalid operation: == (mismatched types int and bool) (1:3)\n | 1 == true\n | ..^", fileError.Error())
}
