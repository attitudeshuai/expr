package optimizer_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/conf"
	"github.com/expr-lang/expr/internal/testify/assert"
	"github.com/expr-lang/expr/internal/testify/require"
	"github.com/expr-lang/expr/optimizer"
	"github.com/expr-lang/expr/parser"
)

func TestRuleNames(t *testing.T) {
	names := optimizer.RuleNames()
	assert.Len(t, names, 14)
	assert.Equal(t, []string{
		"in_array",
		"fold",
		"const_expr",
		"in_range",
		"filter_map",
		"filter_len",
		"filter_last",
		"filter_first",
		"predicate_combination",
		"sum_range",
		"sum_array",
		"sum_map",
		"count_any",
		"count_threshold",
	}, names)

	defaults := optimizer.DefaultRules()
	assert.Len(t, defaults, len(names))
	for _, name := range names {
		assert.True(t, defaults[name], "default set must contain %q", name)
	}

	// DefaultRules returns a mutable copy.
	defaults["fold"] = false
	assert.True(t, optimizer.DefaultRules()["fold"])
}

func TestDisableRuleByName(t *testing.T) {
	t.Run("fold only", func(t *testing.T) {
		code := `1 + 2`

		program, err := expr.Compile(code)
		require.NoError(t, err)
		_, ok := program.Node().(*ast.IntegerNode)
		require.True(t, ok, "expected folded IntegerNode, got %T", program.Node())

		program, err = expr.Compile(code, expr.DisableOptimizationRule("fold"))
		require.NoError(t, err)
		binary, ok := program.Node().(*ast.BinaryNode)
		require.True(t, ok, "expected BinaryNode, got %T", program.Node())
		assert.Equal(t, "+", binary.Operator)

		out, err := expr.Run(program, nil)
		require.NoError(t, err)
		assert.Equal(t, 3, out)
	})

	t.Run("in_array only", func(t *testing.T) {
		code := `v in [1, 2]`
		envOpt := expr.Env(map[string]int{"v": 0})

		program, err := expr.Compile(code, envOpt)
		require.NoError(t, err)
		binary, ok := program.Node().(*ast.BinaryNode)
		require.True(t, ok)
		constant, ok := binary.Right.(*ast.ConstantNode)
		require.True(t, ok, "expected ConstantNode, got %T", binary.Right)
		_, isMap := constant.Value.(map[int]struct{})
		assert.True(t, isMap, "expected map constant, got %T", constant.Value)

		// With only in_array disabled, fold still folds the literal array
		// into a slice constant, but the map rewrite must be gone.
		program, err = expr.Compile(code, envOpt, expr.DisableOptimizationRule("in_array"))
		require.NoError(t, err)
		binary, ok = program.Node().(*ast.BinaryNode)
		require.True(t, ok)
		constant, ok = binary.Right.(*ast.ConstantNode)
		require.True(t, ok, "expected ConstantNode, got %T", binary.Right)
		_, isSlice := constant.Value.([]any)
		assert.True(t, isSlice, "expected slice constant, got %T", constant.Value)

		// With in_array and fold disabled the array node is untouched.
		program, err = expr.Compile(code, envOpt, expr.DisableOptimizationRule("in_array", "fold"))
		require.NoError(t, err)
		binary, ok = program.Node().(*ast.BinaryNode)
		require.True(t, ok)
		_, ok = binary.Right.(*ast.ArrayNode)
		assert.True(t, ok, "expected ArrayNode, got %T", binary.Right)
	})
}

func TestUnknownRuleFailsBeforeCompile(t *testing.T) {
	_, err := expr.Compile(`1 + 2`, expr.OptimizationRules("fild"))
	require.Error(t, err)
	message := err.Error()
	assert.True(t, strings.Contains(message, `unknown optimization rule "fild"`), message)
	for _, name := range optimizer.RuleNames() {
		assert.True(t, strings.Contains(message, name), "error must list %q: %s", name, message)
	}

	// Validation runs before parsing, so a syntactically invalid expression
	// still surfaces the rule-name error.
	_, err = expr.Compile(`1 +`, expr.DisableOptimizationRule("nope"))
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), `unknown optimization rule "nope"`), err.Error())
}

func TestAllRulesDisabledEqualsOptimizeOff(t *testing.T) {
	env := map[string]any{
		"a":     true,
		"b":     false,
		"c":     true,
		"v":     20,
		"users": []map[string]any{{"Age": 20}},
		"items": []map[string]any{{"x": 3}},
		"x":     []int{1, 2, 3},
	}
	codes := []string{
		`1 + 2`,
		`a && b || c`,
		`a ?? b ?? c`,
		`a ? b : c`,
		`v in [1, 2, 3]`,
		`v in 18..31`,
		`all(1..3, # > 0) && any(x, # > 1)`,
		`len(filter(users, .Age > 18))`,
		`filter(users, .Age > 18)[0]`,
		`first(map(filter(users, .Age > 18), .Age))`,
		`count(items, .x) > 2`,
		`sum([1, 2, 3])`,
	}
	for _, code := range codes {
		t.Run(code, func(t *testing.T) {
			off, err := expr.Compile(code, expr.Env(env), expr.Optimize(false))
			require.NoError(t, err)

			allDisabled, err := expr.Compile(code, expr.Env(env), expr.OptimizationRules())
			require.NoError(t, err)

			assert.Equal(t, off.Disassemble(), allDisabled.Disassemble())
		})
	}
}

func TestOptimizationTrace(t *testing.T) {
	code := `v in [1, 2]`
	trace := optimizer.NewTrace()
	_, err := expr.Compile(code, expr.Env(map[string]int{"v": 0}), expr.WithOptimizationTrace(trace))
	require.NoError(t, err)

	report := trace.Report()
	require.NotNil(t, report)

	// Every enabled rule appears, including the skipped const_expr rule,
	// in chain execution order.
	names := make([]string, 0, len(report.Rules))
	for _, rule := range report.Rules {
		names = append(names, rule.Name)
	}
	assert.Equal(t, optimizer.RuleNames(), names)

	inArray, ok := report.Rule("in_array")
	require.True(t, ok)
	assert.Equal(t, 1, inArray.Hits)
	assert.Equal(t, 0, inArray.Rounds)
	assert.Equal(t, "", inArray.ExitReason)
	rewrite := inArray.Rewrites[0]
	assert.Equal(t, "in_array", rewrite.Rule)
	assert.Equal(t, 0, rewrite.Round)
	assert.True(t, rewrite.From >= 0 && rewrite.From < rewrite.To && rewrite.To <= len(code))
	assert.Equal(t, "binary(in)", rewrite.Before)
	assert.Equal(t, "binary(in)", rewrite.After)

	inRange, ok := report.Rule("in_range")
	require.True(t, ok)
	assert.Equal(t, 0, inRange.Rounds)
	assert.Equal(t, "", inRange.ExitReason)

	constRule, ok := report.Rule("const_expr")
	require.True(t, ok)
	assert.Equal(t, 0, constRule.Hits)
	assert.Equal(t, 0, constRule.Rounds)
	assert.Equal(t, optimizer.ExitSkipped, constRule.ExitReason)

	// A fold-heavy expression records fixpoint rounds and per-rewrite rounds.
	foldTrace := optimizer.NewTrace()
	_, err = expr.Compile(`1 + 2`, expr.WithOptimizationTrace(foldTrace))
	require.NoError(t, err)
	fold, ok := foldTrace.Report().Rule("fold")
	require.True(t, ok)
	assert.True(t, fold.Hits >= 1)
	assert.Equal(t, optimizer.ExitConverged, fold.ExitReason)
	assert.True(t, fold.Rounds >= 2, "fold needs at least one applying and one converging round, got %d", fold.Rounds)
	for _, rewrite := range fold.Rewrites {
		assert.Equal(t, "fold", rewrite.Rule)
		assert.True(t, rewrite.Round >= 1)
	}
}

func TestOptimizationTraceConstExpr(t *testing.T) {
	env := map[string]any{"toUpper": strings.ToUpper}
	trace := optimizer.NewTrace()
	_, err := expr.Compile(`toUpper("hi")`,
		expr.Env(env),
		expr.ConstExpr("toUpper"),
		expr.WithOptimizationTrace(trace),
	)
	require.NoError(t, err)

	constRule, ok := trace.Report().Rule("const_expr")
	require.True(t, ok)
	assert.Equal(t, 1, constRule.Hits)
	assert.Equal(t, optimizer.ExitConverged, constRule.ExitReason)
	assert.Equal(t, 2, constRule.Rounds)
	rewrite := constRule.Rewrites[0]
	assert.Equal(t, "const_expr", rewrite.Rule)
	assert.Equal(t, 1, rewrite.Round)
	assert.Equal(t, "call", rewrite.Before)
	assert.Equal(t, "constant(string)", rewrite.After)
}

func TestOptimizationTraceMutation(t *testing.T) {
	tree, err := parser.Parse(`count(items, .active) > 5`)
	require.NoError(t, err)

	trace := optimizer.NewTrace()
	config := conf.CreateNew()
	config.OptimizeTrace = trace
	require.NoError(t, optimizer.Optimize(&tree.Node, config))

	rule, ok := trace.Report().Rule("count_threshold")
	require.True(t, ok)
	assert.Equal(t, 1, rule.Hits)
	rewrite := rule.Rewrites[0]
	assert.Equal(t, "count_threshold", rewrite.Rule)
	assert.Equal(t, "builtin(count)", rewrite.Before)
	assert.Equal(t, "builtin(count){threshold=6}", rewrite.After)
}

func TestOptimizationTraceDeterministic(t *testing.T) {
	env := map[string]any{
		"users": []map[string]any{{"Age": 20}},
		"items": []map[string]any{{"x": 3}},
	}
	code := `len(filter(users, .Age > 18)) > 0 && count(items, .x) > 2 || all(1..3, # > 0)`

	trace1 := optimizer.NewTrace()
	_, err := expr.Compile(code, expr.Env(env), expr.WithOptimizationTrace(trace1))
	require.NoError(t, err)

	trace2 := optimizer.NewTrace()
	_, err = expr.Compile(code, expr.Env(env), expr.WithOptimizationTrace(trace2))
	require.NoError(t, err)

	report1 := trace1.Report()
	report2 := trace2.Report()
	assert.True(t, reflect.DeepEqual(report1, report2), "same expression must produce identical attribution")

	encoded1, err := json.Marshal(report1)
	require.NoError(t, err)
	encoded2, err := json.Marshal(report2)
	require.NoError(t, err)
	assert.Equal(t, string(encoded1), string(encoded2))
	// Reports must not contain pointer-like unstable content.
	assert.False(t, strings.Contains(string(encoded1), "0x"), string(encoded1))
}

func TestConstExprRuleConflict(t *testing.T) {
	env := map[string]any{"toUpper": strings.ToUpper}

	_, err := expr.Compile(`toUpper("hi")`,
		expr.Env(env),
		expr.ConstExpr("toUpper"),
		expr.DisableOptimizationRule("const_expr"),
	)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "const_expr"), err.Error())
	assert.True(t, strings.Contains(err.Error(), "toUpper"), err.Error())

	_, err = expr.Compile(`toUpper("hi")`,
		expr.Env(env),
		expr.ConstExpr("toUpper"),
		expr.OptimizationRules("fold"),
	)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "const_expr"), err.Error())

	// No conflict when the rule stays enabled.
	_, err = expr.Compile(`toUpper("hi")`,
		expr.Env(env),
		expr.ConstExpr("toUpper"),
	)
	require.NoError(t, err)

	// No conflict when const functions are declared but optimization is off.
	_, err = expr.Compile(`toUpper("hi")`,
		expr.Env(env),
		expr.ConstExpr("toUpper"),
		expr.Optimize(false),
	)
	require.NoError(t, err)
}

func TestDefaultAndExplicitRuleSetsProduceSameBytecode(t *testing.T) {
	env := map[string]any{
		"v":     1,
		"users": []map[string]any{{"Age": 20}},
		"items": []map[string]any{{"x": 3}},
	}
	codes := []string{
		`1 + 2 * 3`,
		`v in [1, 2]`,
		`v in 1..10`,
		`len(filter(users, .Age > 18))`,
		`count(items, .x) >= 5`,
		`sum(1..10)`,
	}
	for _, code := range codes {
		t.Run(code, func(t *testing.T) {
			def, err := expr.Compile(code, expr.Env(env))
			require.NoError(t, err)

			explicit, err := expr.Compile(code, expr.Env(env), expr.OptimizationRules(optimizer.RuleNames()...))
			require.NoError(t, err)
			assert.Equal(t, def.Disassemble(), explicit.Disassemble())

			// Attribution must not change the compiled output.
			traced, err := expr.Compile(code, expr.Env(env), expr.WithOptimizationTrace(optimizer.NewTrace()))
			require.NoError(t, err)
			assert.Equal(t, def.Disassemble(), traced.Disassemble())
		})
	}
}

func TestOptimizeDirectCallKeepsWorking(t *testing.T) {
	tree, err := parser.Parse(`1 + 2`)
	require.NoError(t, err)

	require.NoError(t, optimizer.Optimize(&tree.Node, nil))
	_, ok := tree.Node.(*ast.IntegerNode)
	assert.True(t, ok)

	// An explicit empty set disables every rule for direct callers too.
	tree, err = parser.Parse(`1 + 2`)
	require.NoError(t, err)
	config := conf.CreateNew()
	config.OptimizeRules = map[string]bool{}
	require.NoError(t, optimizer.Optimize(&tree.Node, config))
	_, ok = tree.Node.(*ast.BinaryNode)
	assert.True(t, ok)
}
