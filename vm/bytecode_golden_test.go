package vm_test

import (
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/builtin"
	"github.com/expr-lang/expr/internal/testify/require"
)

var updateBytecodeGolden = flag.Bool("updategolden", false, "Update builtin bytecode golden file")

// primaryBytecodeCases contains at least one expression for every builtin
// registered in builtin.Builtins. The corpus guards bytecode stability:
// refactoring the registry plumbing must not change emitted bytecode.
var primaryBytecodeCases = map[string]string{
	// Predicate builtins (dedicated compiler paths).
	"all":           `all([1, 2, 3], # > 0)`,
	"none":          `none([1, 2, 3], # > 3)`,
	"any":           `any([1, 2, 3], # > 2)`,
	"one":           `one([1, 2, 3], # == 2)`,
	"filter":        `filter([1, 2, 3], # > 1)`,
	"map":           `map([1, 2, 3], # * 2)`,
	"find":          `find([1, 2, 3], # > 1)`,
	"findIndex":     `findIndex([1, 2, 3], # > 1)`,
	"findLast":      `findLast([1, 2, 3], # > 1)`,
	"findLastIndex": `findLastIndex([1, 2, 3], # > 1)`,
	"count":         `count([1, 2, 3], # > 1)`,
	"sum":           `sum(1..3, #)`,
	"groupBy":       `groupBy([1, 2, 3], # % 2)`,
	"sortBy":        `sortBy([3, 1, 2], #)`,
	"reduce":        `reduce(1..3, #acc + #, 0)`,

	// Fast builtins (OpCallBuiltin1).
	"len":    `len("foo")`,
	"type":   `type(1)`,
	"abs":    `abs(-1)`,
	"ceil":   `ceil(1.2)`,
	"floor":  `floor(1.2)`,
	"round":  `round(1.2)`,
	"int":    `int("1")`,
	"float":  `float("1")`,
	"string": `string(1)`,

	// Regular function builtins.
	"trim":        `trim(" a ", "a")`,
	"trimPrefix":  `trimPrefix("ab", "a")`,
	"trimSuffix":  `trimSuffix("ab", "b")`,
	"upper":       `upper("a")`,
	"lower":       `lower("A")`,
	"split":       `split("a,b,c", ",", 2)`,
	"splitAfter":  `splitAfter("a,b", ",")`,
	"replace":     `replace("a,b,c", ",", ";", 1)`,
	"indexOf":     `indexOf("abc", "b")`,
	"lastIndexOf": `lastIndexOf("abc", "b")`,
	"hasPrefix":   `hasPrefix("abc", "a")`,
	"hasSuffix":   `hasSuffix("abc", "c")`,
	"max":         `max(1, 2, 3, 4)`,
	"min":         `min(1, 2)`,
	"mean":        `mean(1, 2, 3)`,
	"median":      `median(1, 2, 3)`,
	"toJSON":      `toJSON({a: 1})`,
	"fromJSON":    `fromJSON("{}")`,
	"toBase64":    `toBase64("a")`,
	"fromBase64":  `fromBase64("YQ==")`,
	"now":         `now()`,
	"duration":    `duration("1h")`,
	"date":        `date("2020-01-01", "2006-01-02", "UTC")`,
	"timezone":    `timezone("UTC")`,
	"first":       `first([1, 2])`,
	"last":        `last([1, 2])`,
	"get":         `get([1, 2], 0)`,
	"take":        `take([1, 2], 1)`,
	"keys":        `keys({a: 1})`,
	"values":      `values({a: 1})`,
	"toPairs":     `toPairs({a: 1})`,
	"fromPairs":   `fromPairs([["a", 1]])`,
	"uniq":        `uniq([1, 1, 2])`,

	// Safe (memory-budgeted) builtins (OpCallSafe).
	"repeat":  `repeat("a", 2)`,
	"reverse": `reverse([1, 2])`,
	"concat":  `concat([1], [2], [3], [4])`,
	"flatten": `flatten([[1], [2]])`,
	"sort":    `sort([3, 1, 2], "desc")`,
	"join":    `join(["a", "b"], ",")`,

	// Bitwise builtins.
	"bitand":  `bitand(1, 2)`,
	"bitor":   `bitor(1, 2)`,
	"bitxor":  `bitxor(1, 2)`,
	"bitnand": `bitnand(1, 2)`,
	"bitshl":  `bitshl(1, 2)`,
	"bitshr":  `bitshr(1, 2)`,
	"bitushr": `bitushr(1, 2)`,
	"bitnot":  `bitnot(1)`,
}

// extraBytecodeCases covers alternate arities and disable/override paths.
var extraBytecodeCases = []struct {
	name string
	code string
	ops  []expr.Option
}{
	{"count_no_predicate", `count([true, false, true])`, nil},
	{"sum_no_predicate", `sum(1..9)`, nil},
	{"sortBy_desc", `sortBy([3, 1, 2], #, "desc")`, nil},
	{"reduce_no_initial", `reduce(1..3, #acc + #)`, nil},
	{"len_array", `len([1, 2, 3])`, nil},
	{"trim_one_arg", `trim(" a ")`, nil},
	{"split_two_args", `split("a,b", ",")`, nil},
	{"replace_three_args", `replace("a,b", ",", ";")`, nil},
	{"join_two_args", `join(["a", "b"], ",")`, nil},
	{"join_one_arg", `join(["a", "b"])`, nil},
	{"date_one_arg", `date("2020-01-01")`, nil},
	{"get_map", `get({a: 1}, "a")`, nil},
	{"sort_asc", `sort([3, 1, 2])`, nil},
	{"disabled_overridden_by_env", `len()`, []expr.Option{
		expr.Env(map[string]any{"len": func() int { return 42 }}),
		expr.DisableBuiltin("len"),
	}},
	{"disabled_overridden_by_function", `len()`, []expr.Option{
		expr.Function("len", func(...any) (any, error) { return 42, nil }, new(func() int)),
		expr.DisableBuiltin("len"),
	}},
	{"force_builtin_call", `::len([1, 2])`, []expr.Option{
		expr.Env(map[string]any{"len": func() int { return 42 }}),
	}},
}

func TestBuiltinBytecodeGolden(t *testing.T) {
	var sections []string

	// Primary cases are emitted in registry order: the section name doubles
	// as the coverage record tying bytecode back to the registry index.
	for _, fn := range builtin.Builtins {
		code, ok := primaryBytecodeCases[fn.Name]
		require.True(t, ok, "missing golden bytecode case for builtin %q", fn.Name)
		sections = append(sections, renderBytecode(t, fn.Name, code, nil))
	}
	require.Equal(t, len(builtin.Builtins), len(primaryBytecodeCases),
		"golden corpus contains cases not present in the registry")

	for _, tc := range extraBytecodeCases {
		sections = append(sections, renderBytecode(t, tc.name, tc.code, tc.ops))
	}

	golden := "testdata/builtin_bytecode.golden"
	got := strings.Join(sections, "\n")

	if *updateBytecodeGolden {
		require.NoError(t, os.WriteFile(golden, []byte(got), 0o644))
		return
	}

	b, err := os.ReadFile(golden)
	require.NoError(t, err)
	require.Equal(t, string(b), got,
		"emitted bytecode drifted; run `go test ./vm/ -run TestBuiltinBytecodeGolden -args -updategolden` only if intentional")
}

func renderBytecode(t *testing.T, name, code string, ops []expr.Option) string {
	t.Helper()
	program, err := expr.Compile(code, ops...)
	require.NoError(t, err, "compile %s: %s", name, code)
	return "=== " + name + " ===\n" + program.Disassemble()
}
