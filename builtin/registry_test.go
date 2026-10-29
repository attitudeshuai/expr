package builtin_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/expr-lang/expr/builtin"
	"github.com/expr-lang/expr/conf"
	"github.com/expr-lang/expr/internal/testify/assert"
	"github.com/expr-lang/expr/internal/testify/require"
)

func sameFunc(a, b func(any) any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

func validRegistryEntry(name string) *builtin.Function {
	return &builtin.Function{
		Name:        name,
		Func:        func(args ...any) (any, error) { return nil, nil },
		Category:    builtin.CategoryArray,
		Description: "desc",
		Signature:   name + "(x)",
		Examples:    []string{name + "([1])"},
	}
}

func TestRegistry_DefaultMetadataComplete(t *testing.T) {
	assert.NoError(t, builtin.ValidateBuiltins(),
		"every registered builtin must declare category, description, signature and examples")
}

func TestRegistry_IndexAndNames(t *testing.T) {
	require.Equal(t, len(builtin.Builtins), len(builtin.Names))
	require.Equal(t, len(builtin.Builtins), len(builtin.Index))

	for i, fn := range builtin.Builtins {
		id, ok := builtin.IndexOf(fn.Name)
		require.True(t, ok)
		assert.Equal(t, i, id)
		assert.Equal(t, fn.Name, builtin.NameOf(id))
		assert.Equal(t, fn.Name, builtin.Names[i])
	}
}

func TestRegistry_TableIsFreshAndAligned(t *testing.T) {
	table := builtin.Table()
	require.Equal(t, len(builtin.Builtins), len(table))
	for _, fn := range builtin.Builtins {
		got, ok := table[fn.Name]
		require.True(t, ok)
		assert.Same(t, fn, got)
	}

	// Mutating a derived table must not affect the registry or other tables.
	table["len"] = nil
	delete(table, "abs")
	table2 := builtin.Table()
	assert.NotNil(t, table2["len"])
	assert.NotNil(t, table2["abs"])
}

func TestRegistry_EnabledIsRegistryMinusDisabled(t *testing.T) {
	disabled := map[string]bool{"len": true, "abs": true, "unknown": true}
	enabled := builtin.Enabled(disabled)

	assert.Equal(t, len(builtin.Builtins)-2, len(enabled))
	_, hasLen := enabled["len"]
	assert.False(t, hasLen)
	_, hasAbs := enabled["abs"]
	assert.False(t, hasAbs)
	assert.NotNil(t, enabled["type"])

	// Registry table is untouched.
	assert.Equal(t, len(builtin.Builtins), len(builtin.Table()))
}

func TestRegistry_FastAtAlignedWithRegistry(t *testing.T) {
	for i, fn := range builtin.Builtins {
		assert.True(t, sameFunc(fn.Fast, builtin.FastAt(i)), "fast implementation at index %d not aligned", i)
	}
}

func TestRegistry_ConfigInstanceTableIsDerived(t *testing.T) {
	c := conf.CreateNew()
	require.Equal(t, len(builtin.Builtins), len(c.Builtins))
	for _, fn := range builtin.Builtins {
		got, ok := c.Builtins[fn.Name]
		require.True(t, ok, "config instance table missing %q", fn.Name)
		assert.Same(t, fn, got)
	}

	// Configs get independent copies; mutating one table must not leak into
	// another config or into the registry.
	delete(c.Builtins, "len")
	c2 := conf.CreateNew()
	assert.NotNil(t, c2.Builtins["len"])
	assert.NotNil(t, builtin.Table()["len"])
}

func TestRegistry_PredicateArgumentsCoversPredicateBuiltins(t *testing.T) {
	predicates := builtin.PredicateArguments()
	var count int
	for _, fn := range builtin.Builtins {
		if fn.Predicate {
			count++
			_, ok := predicates[fn.Name]
			assert.True(t, ok, "predicate %q missing from derived table", fn.Name)
		} else {
			_, ok := predicates[fn.Name]
			assert.False(t, ok, "non-predicate %q present in derived table", fn.Name)
		}
	}
	assert.Equal(t, 15, count)
}

func TestRegistry_ValidateAcceptsCompleteEntry(t *testing.T) {
	fn := validRegistryEntry("custom")
	fn.Predicate = true
	fn.Arguments = []builtin.ArgumentSpec{
		{Kind: builtin.ArgumentExpression},
		{Kind: builtin.ArgumentPredicate},
		{Kind: builtin.ArgumentExpression, Optional: true},
	}
	assert.NoError(t, builtin.Validate([]*builtin.Function{fn}))
}

func TestRegistry_ValidateRejectsMissingMetadata(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*builtin.Function)
		want   string
	}{
		{"empty name", func(f *builtin.Function) { f.Name = "" }, "empty name"},
		{"missing category", func(f *builtin.Function) { f.Category = "" }, "missing category"},
		{"unknown category", func(f *builtin.Function) { f.Category = "Nope" }, "unknown category"},
		{"missing description", func(f *builtin.Function) { f.Description = "" }, "missing description"},
		{"missing signature", func(f *builtin.Function) { f.Signature = "" }, "missing signature"},
		{"bad signature prefix", func(f *builtin.Function) { f.Signature = "other(x)" }, "must start with"},
		{"missing examples", func(f *builtin.Function) { f.Examples = nil }, "missing examples"},
		{"blank example", func(f *builtin.Function) { f.Examples = []string{"  "} }, "example 1 is empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fn := validRegistryEntry("custom")
			tc.mutate(fn)
			err := builtin.Validate([]*builtin.Function{fn})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestRegistry_ValidateRejectsNilEntry(t *testing.T) {
	err := builtin.Validate([]*builtin.Function{nil})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil entry")
}

func TestRegistry_ValidateRejectsDuplicateNames(t *testing.T) {
	err := builtin.Validate([]*builtin.Function{
		validRegistryEntry("dup"),
		validRegistryEntry("dup"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate name")
}

func TestRegistry_ValidatePredicateShapes(t *testing.T) {
	predicate := func(args []builtin.ArgumentSpec) *builtin.Function {
		fn := validRegistryEntry("custom")
		fn.Predicate = true
		fn.Arguments = args
		return fn
	}

	t.Run("predicate without specs", func(t *testing.T) {
		err := builtin.Validate([]*builtin.Function{predicate(nil)})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must declare argument specs")
	})

	t.Run("non-predicate with specs", func(t *testing.T) {
		fn := validRegistryEntry("custom")
		fn.Arguments = []builtin.ArgumentSpec{{Kind: builtin.ArgumentExpression}}
		err := builtin.Validate([]*builtin.Function{fn})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must not declare argument specs")
	})

	t.Run("without predicate argument", func(t *testing.T) {
		err := builtin.Validate([]*builtin.Function{predicate([]builtin.ArgumentSpec{
			{Kind: builtin.ArgumentExpression},
		})})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exactly one predicate argument")
	})

	t.Run("required after optional", func(t *testing.T) {
		err := builtin.Validate([]*builtin.Function{predicate([]builtin.ArgumentSpec{
			{Kind: builtin.ArgumentExpression},
			{Kind: builtin.ArgumentPredicate, Optional: true},
			{Kind: builtin.ArgumentExpression},
		})})
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "follows an optional one"))
	})
}
