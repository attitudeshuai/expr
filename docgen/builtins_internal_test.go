package docgen

import (
	"reflect"
	"testing"

	"github.com/expr-lang/expr/builtin"
)

func countBuiltins() int {
	return len(builtin.Builtins)
}

func TestBuiltinsDerivedFromRegistry(t *testing.T) {
	rederived := builtinTypes()

	if len(Builtins) != countBuiltins() {
		t.Fatalf("derived table has %d entries, registry %d", len(Builtins), countBuiltins())
	}
	if len(rederived) != len(Builtins) {
		t.Fatalf("re-derived table has %d entries, package table %d", len(rederived), len(Builtins))
	}

	for _, fn := range builtin.Builtins {
		got, ok := Builtins[Identifier(fn.Name)]
		if !ok {
			t.Errorf("derived table missing builtin %q", fn.Name)
			continue
		}
		if got.Kind != "func" {
			t.Errorf("builtin %q: kind = %q, want func", fn.Name, got.Kind)
		}
		if !reflect.DeepEqual(got, rederived[Identifier(fn.Name)]) {
			t.Errorf("builtin %q: package table drifted from fresh derivation", fn.Name)
		}
	}

	for name := range Builtins {
		if _, ok := rederived[name]; !ok {
			t.Errorf("package table contains extra %q", name)
		}
	}
}

func TestBuiltinTypesSpecialForms(t *testing.T) {
	cases := []struct {
		name       string
		args       int
		returnKind Kind
	}{
		// Fast with Validate.
		{"len", 1, "int"},
		{"abs", 1, "any"},
		{"ceil", 1, "float"},
		{"int", 1, "int"},
		// Fast with Types.
		{"type", 1, "string"},
		{"string", 1, "string"},
		// Validate-only variadic aggregates.
		{"max", 1, "any"},
		{"mean", 1, "any"},
		{"concat", 1, "array"},
		// Validate-only returns.
		{"keys", 1, "array"},
		{"fromPairs", 1, "map"},
		{"first", 1, "any"},
		{"take", 2, "any"},
		// Zero-arity Validate-only.
		{"now", 0, "struct"},
		// get: neither Types nor Validate.
		{"get", 2, "any"},
		// Predicates.
		{"all", 2, "bool"},
		{"filter", 2, "array"},
		{"count", 2, "int"},
		{"groupBy", 2, "map"},
		{"reduce", 3, "any"},
		// Types-based funcs.
		{"duration", 1, "time.Duration"},
		{"bitand", 2, "int"},
		{"trim", 1, "string"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fn, ok := Builtins[Identifier(tc.name)]
			if !ok {
				t.Fatalf("missing %q", tc.name)
			}
			if len(fn.Arguments) != tc.args {
				t.Errorf("%q args = %d, want %d", tc.name, len(fn.Arguments), tc.args)
			}
			if fn.Return == nil {
				t.Fatalf("%q has no return type", tc.name)
			}
			if fn.Return.Kind != tc.returnKind {
				t.Errorf("%q return = %q, want %q", tc.name, fn.Return.Kind, tc.returnKind)
			}
		})
	}
}

func TestBuiltinCatalogCoversRegistry(t *testing.T) {
	catalog := builtinCatalog()
	var count int
	seen := make(map[string]bool)
	for _, category := range catalog {
		for _, fn := range category.Functions {
			count++
			seen[fn.Name] = true
			if fn.Signature == "" || len(fn.Examples) == 0 || fn.Description == "" || fn.Type == nil {
				t.Errorf("catalog entry %q incomplete", fn.Name)
			}
		}
	}
	if count != countBuiltins() {
		t.Fatalf("catalog covers %d functions, registry has %d", count, countBuiltins())
	}
	for _, fn := range builtin.Builtins {
		if !seen[fn.Name] {
			t.Errorf("catalog missing %q", fn.Name)
		}
	}
}
