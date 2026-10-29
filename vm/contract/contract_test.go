package contract_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/expr-lang/expr/conf"
	"github.com/expr-lang/expr/internal/testify/assert"
	"github.com/expr-lang/expr/internal/testify/require"
	"github.com/expr-lang/expr/vm/contract"
)

type embeddedHelpers struct {
	Promoted string
}

func (embeddedHelpers) Help() string { return "help" }

type contractEnv struct {
	Name   string
	Count  int
	Ratio  float64
	Items  []string
	Alias  string `expr:"renamed"`
	Hidden string `expr:"-"`
	embeddedHelpers
}

func (contractEnv) Greet() string { return "hi" }

func buildContract(t *testing.T, env any) *contract.Contract {
	t.Helper()
	c := conf.New(env)
	ct := conf.NewEnvContract(c.Env, &c.NtCache)
	require.NotNil(t, ct)
	return ct
}

func TestNewEnvContract_Struct(t *testing.T) {
	c := buildContract(t, contractEnv{})

	assert.Equal(t, contract.KindStruct, c.Kind)
	assert.True(t, c.Strict)
	assert.False(t, c.FastMap)

	// Effective names: own fields, promoted embedded fields and methods,
	// renamed fields under effective names; hidden fields excluded.
	assert.Contains(t, c.Names, "Name")
	assert.Contains(t, c.Names, "Count")
	assert.Contains(t, c.Names, "Items")
	assert.Contains(t, c.Names, "renamed")
	assert.Contains(t, c.Names, "Promoted")
	assert.Contains(t, c.Names, "Help")
	assert.Contains(t, c.Names, "Greet")
	assert.NotContains(t, c.Names, "Alias")
	assert.NotContains(t, c.Names, "Hidden")
	assert.NotContains(t, c.Names, "embeddedHelpers")
	assert.True(t, c.Names["Greet"].Func)
	assert.Equal(t, "slice", c.Names["Items"].Kind.String())
}

func TestNewEnvContract_Map(t *testing.T) {
	env := map[string]any{
		"n":  1,
		"s":  "x",
		"fn": func() {},
	}
	c := buildContract(t, env)

	assert.Equal(t, contract.KindMap, c.Kind)
	assert.True(t, c.FastMap)
	assert.True(t, c.Strict)
	assert.Contains(t, c.Names, "n")
	assert.Contains(t, c.Names, "s")
	assert.True(t, c.Names["fn"].Func)

	// Typed maps are not fast maps.
	typed := buildContract(t, map[string]string{"a": "b"})
	assert.False(t, typed.FastMap)
}

func TestNewEnvContract_None(t *testing.T) {
	c := conf.CreateNew()
	assert.Nil(t, conf.NewEnvContract(c.Env, &c.NtCache))
}

func TestContract_Validate_StructPasses(t *testing.T) {
	c := buildContract(t, contractEnv{})
	assert.NoError(t, c.Validate(contractEnv{}))
	// Pointer to the same struct kind passes as well.
	assert.NoError(t, c.Validate(&contractEnv{}))
}

func TestContract_Validate_NilAndWrongKind(t *testing.T) {
	c := buildContract(t, contractEnv{})

	err := c.Validate(nil)
	require.Error(t, err)
	var mm *contract.MismatchError
	require.True(t, errors.As(err, &mm))
	assert.Contains(t, err.Error(), "expected struct environment, got nil")

	err = c.Validate(map[string]any{"Name": "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected struct environment, got map")

	mc := buildContract(t, map[string]any{"a": 1})
	err = mc.Validate(contractEnv{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected map environment, got struct")
}

type missingEnv struct {
	Name string
}

type mismatchEnv struct {
	Name  int
	Count int
}

func TestContract_Validate_MissingAndMismatchedNames(t *testing.T) {
	c := buildContract(t, contractEnv{})

	err := c.Validate(missingEnv{})
	require.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "env contract mismatch")
	assert.Contains(t, msg, "missing names: ")
	// Several contracted names must be listed in a single aggregated error.
	assert.Contains(t, msg, "Count")
	assert.Contains(t, msg, "Items")
	assert.Contains(t, msg, "renamed")
	// Names are sorted.
	assert.Less(t, strings.Index(msg, "Count"), strings.Index(msg, "Items"))

	err = c.Validate(mismatchEnv{})
	require.Error(t, err)
	msg = err.Error()
	assert.Contains(t, msg, "mismatched names: Name (expected string, got int)")
}

func TestContract_Validate_Map(t *testing.T) {
	env := map[string]any{
		"n": 1,
		"s": "x",
	}
	c := buildContract(t, env)

	assert.NoError(t, c.Validate(map[string]any{"n": 2, "s": "y"}))
	// Extra names are allowed.
	assert.NoError(t, c.Validate(map[string]any{"n": 2, "s": "y", "extra": true}))

	err := c.Validate(map[string]any{"n": 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing names: s")

	err = c.Validate(map[string]any{"n": "1", "s": "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mismatched names: n (expected int, got string)")

	err = c.Validate(map[string]any{"n": nil, "s": "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "n (expected int, got nil)")
}

func TestContract_Validate_NilValueIsAny(t *testing.T) {
	env := map[string]any{"x": nil}
	c := buildContract(t, env)
	assert.NoError(t, c.Validate(map[string]any{"x": 42}))
	assert.NoError(t, c.Validate(map[string]any{"x": "anything"}))
}

func TestContract_Validate_LooseContract(t *testing.T) {
	// A loose contract contains only declared names; undeclared names are
	// simply absent and therefore never validated.
	c := &contract.Contract{
		Kind:    contract.KindMap,
		Strict:  false,
		FastMap: true,
		Names: map[string]*contract.Shape{
			"a": {Kind: reflect.Int},
		},
	}
	assert.NoError(t, c.Validate(map[string]any{"a": 1}))
	assert.NoError(t, c.Validate(map[string]any{"a": 1, "undeclared": "ok"}))
	err := c.Validate(map[string]any{"undeclared": "ok"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing names: a")
}

func TestContract_Key(t *testing.T) {
	c1 := buildContract(t, contractEnv{})
	c2 := buildContract(t, contractEnv{})
	assert.NotEmpty(t, c1.Key())
	assert.Equal(t, c1.Key(), c2.Key())

	other := buildContract(t, mismatchEnv{})
	assert.NotEqual(t, c1.Key(), other.Key())

	m1 := buildContract(t, map[string]any{"a": 1})
	m2 := buildContract(t, map[string]any{"a": 1, "b": 2})
	assert.NotEqual(t, m1.Key(), m2.Key())
}
