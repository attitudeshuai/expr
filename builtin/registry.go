package builtin

import (
	"fmt"
	"strings"
)

var (
	// Index maps a builtin name to its global registry index. The index is the
	// operand of OpCallBuiltin1 and therefore doubles as the runtime index; it
	// must remain stable across releases.
	Index map[string]int
	// Names lists builtin names in registry order.
	Names []string

	// fastFunctions is aligned with Builtins: entry i is Builtins[i].Fast
	// (nil for non-fast builtins). The VM reads fast implementations through
	// this derived slice instead of poking registry struct fields.
	fastFunctions []func(any) any
)

// CategoryOrder lists documentation categories in their canonical order.
var CategoryOrder = []Category{
	CategoryString,
	CategoryDate,
	CategoryNumber,
	CategoryArray,
	CategoryMap,
	CategoryConversion,
	CategoryMiscellaneous,
	CategoryBitwise,
}

func init() {
	Index, Names, fastFunctions = derive(Builtins)
}

// derive computes all registry-wide derived views in one pass.
func derive(fns []*Function) (map[string]int, []string, []func(any) any) {
	index := make(map[string]int, len(fns))
	names := make([]string, len(fns))
	fast := make([]func(any) any, len(fns))
	for i, fn := range fns {
		index[fn.Name] = i
		names[i] = fn.Name
		fast[i] = fn.Fast
	}
	return index, names, fast
}

// Table returns a fresh name to function map covering the whole registry.
// Callers may mutate the returned map (for example to drop disabled entries)
// without affecting the registry.
func Table() map[string]*Function {
	table := make(map[string]*Function, len(Builtins))
	for _, fn := range Builtins {
		table[fn.Name] = fn
	}
	return table
}

// Enabled returns a fresh name to function table containing every registry
// function except the disabled ones.
func Enabled(disabled map[string]bool) map[string]*Function {
	table := Table()
	for name := range disabled {
		delete(table, name)
	}
	return table
}

// NameOf returns the builtin name at the given global registry index.
func NameOf(index int) string {
	return Names[index]
}

// IndexOf returns the global registry index of a builtin.
func IndexOf(name string) (int, bool) {
	i, ok := Index[name]
	return i, ok
}

// ByIndex returns the registered function at the given global registry index.
func ByIndex(index int) *Function {
	return Builtins[index]
}

// FastAt returns the fast implementation at the given global registry index
// (nil for builtins without one; OpCallBuiltin1 is only emitted when non-nil).
func FastAt(index int) func(any) any {
	return fastFunctions[index]
}

// PredicateArguments returns a fresh name to argument-shape table containing
// every predicate builtin. The parser builds its grammar table from it.
func PredicateArguments() map[string][]ArgumentSpec {
	out := make(map[string][]ArgumentSpec)
	for _, fn := range Builtins {
		if fn.Predicate {
			specs := make([]ArgumentSpec, len(fn.Arguments))
			copy(specs, fn.Arguments)
			out[fn.Name] = specs
		}
	}
	return out
}

// Validate checks that the given function registrations are complete and
// self-consistent. It validates metadata completeness (category, description,
// signature, examples) and predicate argument shapes. It is exported so that
// tests can exercise it with synthetic entries.
func Validate(fns []*Function) error {
	categories := make(map[Category]bool, len(CategoryOrder))
	for _, c := range CategoryOrder {
		categories[c] = true
	}

	var problems []string
	seen := make(map[string]bool, len(fns))

	for i, fn := range fns {
		at := fmt.Sprintf("builtin at index %d", i)
		if fn != nil {
			at = fmt.Sprintf("builtin %q", fn.Name)
		}
		if fn == nil {
			problems = append(problems, at+": nil entry")
			continue
		}
		if fn.Name == "" {
			problems = append(problems, at+": empty name")
		} else if seen[fn.Name] {
			problems = append(problems, fmt.Sprintf("builtin %q: duplicate name", fn.Name))
		}
		seen[fn.Name] = true

		if fn.Category == "" {
			problems = append(problems, fmt.Sprintf("builtin %q: missing category", fn.Name))
		} else if !categories[fn.Category] {
			problems = append(problems, fmt.Sprintf("builtin %q: unknown category %q", fn.Name, fn.Category))
		}
		if strings.TrimSpace(fn.Description) == "" {
			problems = append(problems, fmt.Sprintf("builtin %q: missing description", fn.Name))
		}
		if fn.Signature == "" {
			problems = append(problems, fmt.Sprintf("builtin %q: missing signature", fn.Name))
		} else if fn.Name != "" && !strings.HasPrefix(fn.Signature, fn.Name+"(") {
			problems = append(problems, fmt.Sprintf("builtin %q: signature %q must start with %q", fn.Name, fn.Signature, fn.Name+"("))
		}
		if len(fn.Examples) == 0 {
			problems = append(problems, fmt.Sprintf("builtin %q: missing examples", fn.Name))
		} else {
			for j, example := range fn.Examples {
				if strings.TrimSpace(example) == "" {
					problems = append(problems, fmt.Sprintf("builtin %q: example %d is empty", fn.Name, j+1))
				}
			}
		}
		validatePredicateArguments(&problems, fn)
	}

	if len(problems) > 0 {
		return fmt.Errorf("builtin registry validation failed:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}

func validatePredicateArguments(problems *[]string, fn *Function) {
	switch {
	case fn.Predicate && len(fn.Arguments) == 0:
		*problems = append(*problems, fmt.Sprintf("builtin %q: predicate builtin must declare argument specs", fn.Name))
	case !fn.Predicate && len(fn.Arguments) != 0:
		*problems = append(*problems, fmt.Sprintf("builtin %q: non-predicate builtin must not declare argument specs", fn.Name))
	case fn.Predicate:
		predicates := 0
		seenOptional := false
		for j, arg := range fn.Arguments {
			if arg.Optional {
				seenOptional = true
			} else if seenOptional {
				*problems = append(*problems, fmt.Sprintf("builtin %q: required argument %d follows an optional one", fn.Name, j+1))
			}
			switch arg.Kind {
			case ArgumentPredicate:
				predicates++
			case ArgumentExpression:
			default:
				*problems = append(*problems, fmt.Sprintf("builtin %q: argument %d has unknown kind", fn.Name, j+1))
			}
		}
		if predicates != 1 {
			*problems = append(*problems, fmt.Sprintf("builtin %q: predicate builtin must declare exactly one predicate argument (got %d)", fn.Name, predicates))
		}
	}
}

// ValidateBuiltins validates the default registry.
func ValidateBuiltins() error {
	return Validate(Builtins)
}
