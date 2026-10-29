package builtin_test

import (
	"testing"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/builtin"
	"github.com/expr-lang/expr/internal/testify/require"
)

// TestRegistry_ExamplesCompile makes every registered example compile against
// a strict empty environment. An unknown identifier, a typo or a wrong arity
// fails compilation here instead of silently rotting in documentation.
func TestRegistry_ExamplesCompile(t *testing.T) {
	for _, fn := range builtin.Builtins {
		fn := fn
		t.Run(fn.Name, func(t *testing.T) {
			require.GreaterOrEqual(t, len(fn.Examples), 1)
			for _, example := range fn.Examples {
				_, err := expr.Compile(example, expr.Env(nil))
				require.NoError(t, err, "example of %q does not compile: %s", fn.Name, example)
			}
		})
	}
}

// TestRegistry_BadExampleFailsCompile proves the example gate rejects
// malformed examples instead of silently accepting them.
func TestRegistry_BadExampleFailsCompile(t *testing.T) {
	for _, example := range []string{
		`len(`,                 // syntax error
		`unknown_ident("foo")`, // unknown name in strict environment
		`abs("foo", "bar")`,    // invalid overload
		`trim()`,               // not enough arguments
	} {
		_, err := expr.Compile(example, expr.Env(nil))
		require.Error(t, err, "bad example unexpectedly compiled: %s", example)
	}
}
