package parser

import (
	"testing"

	"github.com/expr-lang/expr/builtin"
)

func TestPredicatesDerivedFromRegistry(t *testing.T) {
	registry := builtin.PredicateArguments()

	if len(predicates) != len(registry) {
		t.Fatalf("derived predicate table has %d entries, registry has %d", len(predicates), len(registry))
	}

	for name, specs := range registry {
		got, ok := predicates[name]
		if !ok {
			t.Errorf("predicate %q missing from derived parser table", name)
			continue
		}
		if len(got.args) != len(specs) {
			t.Errorf("predicate %q: derived arity %d, registry arity %d", name, len(got.args), len(specs))
			continue
		}
		for i, spec := range specs {
			var want arg
			if spec.Kind == builtin.ArgumentPredicate {
				want = predicate
			} else {
				want = expr
			}
			if spec.Optional {
				want |= optional
			}
			if got.args[i] != want {
				t.Errorf("predicate %q arg %d: derived flag %d, registry expects %d", name, i, got.args[i], want)
			}
		}
	}

	for name := range predicates {
		if _, ok := registry[name]; !ok {
			t.Errorf("derived table contains %q which is not a registry predicate", name)
		}
	}

	// Lock the predicate grammar shape for functions with optional arguments.
	if predicates["count"].args[1]&optional == 0 {
		t.Error("count predicate argument must be optional")
	}
	if predicates["sum"].args[1]&optional == 0 {
		t.Error("sum predicate argument must be optional")
	}
	if predicates["sortBy"].args[2]&optional == 0 {
		t.Error("sortBy order argument must be optional")
	}
	if predicates["reduce"].args[2]&optional == 0 {
		t.Error("reduce initialValue argument must be optional")
	}
}
