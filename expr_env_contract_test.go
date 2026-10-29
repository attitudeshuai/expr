package expr

import (
	"errors"
	"strings"
	"testing"

	"github.com/expr-lang/expr/internal/testify/assert"
	"github.com/expr-lang/expr/internal/testify/require"
	"github.com/expr-lang/expr/types"
	"github.com/expr-lang/expr/vm"
	"github.com/expr-lang/expr/vm/contract"
)

type ecEmbed struct {
	Embedded string
}

type ecEnv struct {
	Name string
	Num  int
	Post struct {
		Title string
	}
	Tags []string
	Map  map[string]string `expr:"tags_map"`
	ecEmbed
}

func (ecEnv) Upper(s string) string { return strings.ToUpper(s) }

type ecEnvMissing struct {
	Name string
}

type ecPostMismatch struct {
	Title int
}

type ecEnvMismatch struct {
	Name string
	Num  string
	Post ecPostMismatch
	Tags []string
}

// ecEnvNested passes entry validation against ecEnv (same top-level names
// and kinds) but carries a differently typed nested Post.Title field.
type ecPostNested struct {
	Title int
}

type ecEnvNested struct {
	Name string
	Num  int
	Post ecPostNested
	Tags []string
	Map  map[string]string `expr:"tags_map"`
	ecEmbed
}

func (ecEnvNested) Upper(s string) string { return s }

func compileWithContract(t *testing.T, code string, env any, opts ...Option) *vm.Program {
	t.Helper()
	all := append([]Option{Env(env), ValidateEnv()}, opts...)
	program, err := Compile(code, all...)
	require.NoError(t, err)
	return program
}

func asMismatch(t *testing.T, err error) *contract.MismatchError {
	t.Helper()
	require.Error(t, err)
	var mm *contract.MismatchError
	require.True(t, errors.As(err, &mm), "expected contract.MismatchError, got %v", err)
	return mm
}

func TestEnvContract_StructHappyPath(t *testing.T) {
	env := ecEnv{Name: "abc", Num: 7, Tags: []string{"a", "b"}}
	program := compileWithContract(t, `Upper(Name) + "-" + Tags[1]`, env)

	out, err := Run(program, env)
	require.NoError(t, err)
	assert.Equal(t, "ABC-b", out)
}

func TestEnvContract_PointerEnvAccepted(t *testing.T) {
	env := ecEnv{Name: "abc"}
	program := compileWithContract(t, `Name`, env)

	out, err := Run(program, &env)
	require.NoError(t, err)
	assert.Equal(t, "abc", out)
}

func TestEnvContract_MissingNamesFailBeforeExecution(t *testing.T) {
	env := ecEnv{}
	program := compileWithContract(t, `Num`, env)

	_, err := Run(program, ecEnvMissing{})
	mm := asMismatch(t, err)

	// All missing names are listed and sorted in a single error.
	assert.Contains(t, mm.Missing, "Num")
	assert.Contains(t, mm.Missing, "Post")
	assert.Contains(t, mm.Missing, "tags_map")
	assert.Contains(t, mm.Missing, "Embedded")
	assert.Contains(t, mm.Missing, "Upper")
	assert.Contains(t, err.Error(), "missing names:")
	// The error is returned before execution, so it carries no source binding.
	assert.NotContains(t, err.Error(), "(1:")
}

func TestEnvContract_ShapeMismatchAtEntry(t *testing.T) {
	env := ecEnv{}
	program := compileWithContract(t, `Num`, env)

	_, err := Run(program, ecEnvMismatch{})
	mm := asMismatch(t, err)
	require.Len(t, mm.Mismatched, 1)
	assert.Equal(t, "Num", mm.Mismatched[0].Name)
	assert.Equal(t, "int", mm.Mismatched[0].Expected)
	assert.Equal(t, "string", mm.Mismatched[0].Got)
}

func TestEnvContract_CrossKindRejected(t *testing.T) {
	env := ecEnv{}
	program := compileWithContract(t, `Num`, env)

	// Struct artifact against a mapping with same-named keys: rejected per
	// the documented "same environment type" rule.
	_, err := Run(program, map[string]any{"Num": 1})
	mm := asMismatch(t, err)
	assert.Equal(t, "struct", mm.Expected)
	assert.Equal(t, "map", mm.Got)
	assert.Contains(t, err.Error(), "expected struct environment, got map")

	// And the reverse: a map artifact against a struct.
	mapEnv := map[string]any{"a": 1}
	mapProgram := compileWithContract(t, `a`, mapEnv)
	_, err = Run(mapProgram, ecEnv{})
	mm = asMismatch(t, err)
	assert.Equal(t, "map", mm.Expected)
	assert.Equal(t, "struct", mm.Got)
}

func TestEnvContract_NestedFieldMismatchFailsAtFetch(t *testing.T) {
	// Top-level shapes match (Post is a struct in both), only the nested
	// field type differs. Entry validation passes, the field gate fails.
	env := ecEnv{}
	program := compileWithContract(t, `Post.Title`, env)

	_, err := Run(program, ecEnvNested{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Post.Title (expected string, got int)")
	// Per-site gate errors happen during execution and get a source binding.
	assert.Contains(t, err.Error(), "(1:")
}

func TestEnvContract_FastMapMissingKeyNoLongerSilent(t *testing.T) {
	declared := map[string]any{"a": 1, "b": 2}
	program := compileWithContract(t, `b + 1`, declared)

	// Without validation the fast path silently reads nil from a missing key.
	_, err := Run(program, map[string]any{"a": 1})
	mm := asMismatch(t, err)
	assert.Contains(t, mm.Missing, "b")

	out, err := Run(program, declared)
	require.NoError(t, err)
	assert.Equal(t, 3, out)
}

func TestEnvContract_LooseMode(t *testing.T) {
	declared := map[string]any{"a": 1}

	// Undeclared names are not part of the contract and keep returning nil.
	program := compileWithContract(t, `unknown`, declared, AllowUndefinedVariables())
	out, err := Run(program, map[string]any{"a": 1})
	require.NoError(t, err)
	assert.Nil(t, out)

	// Declared names are still enforced before execution.
	programDeclared := compileWithContract(t, `a`, declared, AllowUndefinedVariables())
	_, err = Run(programDeclared, map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing names: a")
}

func TestEnvContract_NestedMapMissingKeyStaysNil(t *testing.T) {
	declared := types.Map{
		"o": types.Map{"list": types.Array(types.Int)},
	}
	program := compileWithContract(t, `o.list`, declared)

	out, err := Run(program, map[string]any{"o": map[string]any{}})
	require.NoError(t, err)
	assert.Nil(t, out)

	// Wrong runtime shape of a nested value fails at the fetch gate.
	_, err = Run(program, map[string]any{"o": map[string]any{"list": 42}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected slice, got int")
}

func TestEnvContract_FirstLast(t *testing.T) {
	env := ecEnv{Tags: []string{"a", "b"}}

	first := compileWithContract(t, `first(Tags)`, ecEnv{})
	out, err := Run(first, env)
	require.NoError(t, err)
	assert.Equal(t, "a", out)
	assert.Contains(t, first.Disassemble(), "OpFetchBuiltin")

	last := compileWithContract(t, `last(Tags)`, ecEnv{})
	out, err = Run(last, env)
	require.NoError(t, err)
	assert.Equal(t, "b", out)

	// Out of range stays nil (matches the builtin semantics).
	empty := compileWithContract(t, `first(Tags)`, ecEnv{})
	out, err = Run(empty, ecEnv{Tags: []string{}})
	require.NoError(t, err)
	assert.Nil(t, out)

	// Unvalidated programs keep the recover-swallowing behavior of the
	// builtin: an any-typed value that is not an array yields nil.
	unvalidated, err := Compile(`first(x)`, Env(types.Map{"x": types.Any}))
	require.NoError(t, err)
	assert.NotContains(t, unvalidated.Disassemble(), "OpFetchBuiltin")
	out, err = Run(unvalidated, map[string]any{"x": 42})
	require.NoError(t, err)
	assert.Nil(t, out)
}

func TestEnvContract_GlobalSwitch(t *testing.T) {
	prev := vm.EnvValidationEnabled
	vm.EnvValidationEnabled = false
	t.Cleanup(func() { vm.EnvValidationEnabled = prev })

	program := compileWithContract(t, `Num`, ecEnv{})
	_, contractErr := Run(program, ecEnvMissing{})
	// Global switch off: no pre-execution contract error; execution either
	// fails with a runtime error or returns a zero value, but never a
	// contract mismatch.
	assert.True(t, contractErr == nil ||
		!strings.Contains(contractErr.Error(), "env contract mismatch"))

	vm.EnvValidationEnabled = true
	_, err := Run(program, ecEnvMissing{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "env contract mismatch")
}

func TestEnvContract_DefaultBehaviorUnchanged(t *testing.T) {
	// Without ValidateEnv the fast map silent nil is preserved verbatim.
	loose, err := Compile(`b`, Env(map[string]any{"a": 1}), AllowUndefinedVariables())
	require.NoError(t, err)
	out, err := Run(loose, map[string]any{"a": 1})
	require.NoError(t, err)
	assert.Nil(t, out)

	// Struct artifact run against a map fails during execution (not before)
	// and the error text is the runtime one, not the contract one.
	structProgram, err := Compile(`Num`, Env(ecEnv{}))
	require.NoError(t, err)
	_, err = Run(structProgram, map[string]any{"Num": 1})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "env contract mismatch")
}

func TestEnvContract_NoEnvSkipsValidation(t *testing.T) {
	// expr.Eval has no env declaration and is unaffected.
	out, err := Eval(`1 + 2`, nil)
	require.NoError(t, err)
	assert.Equal(t, 3, out)

	// A program compiled without Env carries no contract.
	program, err := Compile(`1 + 2`)
	require.NoError(t, err)
	assert.Nil(t, program.EnvContract())
}

func TestEnvContract_AllEntryPointsShareValidation(t *testing.T) {
	program := compileWithContract(t, `Num`, ecEnv{})

	// expr.Run, vm.Run and (*vm.VM).Run all reject before execution.
	for name, run := range map[string]func() (any, error){
		"expr.Run":  func() (any, error) { return Run(program, ecEnvMissing{}) },
		"vm.Run":    func() (any, error) { return vm.Run(program, ecEnvMissing{}) },
		"(*VM).Run": func() (any, error) { return (&vm.VM{}).Run(program, ecEnvMissing{}) },
	} {
		_, err := run()
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), "env contract mismatch", name)
	}

	// All accept a valid environment.
	_, err := (&vm.VM{}).Run(program, ecEnv{Num: 42})
	require.NoError(t, err)
}

func TestEnvContract_ContractRetainedAndFingerprinted(t *testing.T) {
	// The contract is retained even when enforcement is not enabled.
	program, err := Compile(`Name`, Env(ecEnv{}))
	require.NoError(t, err)
	c := program.EnvContract()
	require.NotNil(t, c)
	assert.Equal(t, contract.KindStruct, c.Kind)
	assert.Contains(t, c.Names, "Name")
	assert.Contains(t, c.Names, "tags_map") // renamed field, effective name
	assert.Contains(t, c.Names, "Embedded")
	assert.Contains(t, c.Names, "Upper")
	assert.NotContains(t, c.Names, "Map")
	assert.NotContains(t, c.Names, "ecEmbed")

	other, err := Compile(`Num`, Env(ecEnvMismatch{}))
	require.NoError(t, err)
	assert.NotEqual(t, program.EnvContract().Key(), other.EnvContract().Key())

	same, err := Compile(`Num`, Env(ecEnv{}))
	require.NoError(t, err)
	assert.Equal(t, program.EnvContract().Key(), same.EnvContract().Key())
}
