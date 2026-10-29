package docgen

import (
	"reflect"

	"github.com/expr-lang/expr/builtin"
	"github.com/expr-lang/expr/internal/deref"
)

// BuiltinCategory groups builtin functions for documentation.
type BuiltinCategory struct {
	Name      string             `json:"name"`
	Functions []*BuiltinFunction `json:"functions"`
}

// BuiltinFunction carries documentation metadata of a builtin function.
// Everything here is derived from the builtin registry, the single source
// of truth.
type BuiltinFunction struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Signature   string   `json:"signature"`
	Examples    []string `json:"examples,omitempty"`
	Type        *Type    `json:"type,omitempty"`
}

// builtinTypes derives the legacy name to type table from the registry. It
// replaces the former hand-maintained table which drifted from the registry.
func builtinTypes() map[Identifier]*Type {
	out := make(map[Identifier]*Type, len(builtin.Builtins))
	for _, fn := range builtin.Builtins {
		out[Identifier(fn.Name)] = builtinFuncType(fn)
	}
	return out
}

// Builtins is derived from the builtin registry. It may be set to nil to
// suppress builtins in documentation output (used by tests).
var Builtins = builtinTypes()

// builtinCatalog groups registry metadata by category in canonical order.
func builtinCatalog() []*BuiltinCategory {
	groups := make(map[builtin.Category]*BuiltinCategory, len(builtin.CategoryOrder))
	categories := make([]*BuiltinCategory, 0, len(builtin.CategoryOrder))
	for _, category := range builtin.CategoryOrder {
		group := &BuiltinCategory{Name: category.Title()}
		groups[category] = group
		categories = append(categories, group)
	}
	for _, fn := range builtin.Builtins {
		group, ok := groups[fn.Category]
		if !ok {
			continue
		}
		group.Functions = append(group.Functions, &BuiltinFunction{
			Name:        fn.Name,
			Description: fn.Description,
			Signature:   fn.Signature,
			Examples:    append([]string(nil), fn.Examples...),
			Type:        builtinFuncType(fn),
		})
	}
	return categories
}

// builtinFuncType converts a registered builtin into its documentation type
// shape. Functions with declared Types use the first signature; Validate-only
// functions are probed with interface arguments to discover their arity and
// documented return kind.
func builtinFuncType(fn *builtin.Function) *Type {
	if len(fn.Types) > 0 {
		return typeOf(fn.Types[0])
	}
	if fn.Name == "get" {
		return &Type{
			Kind:      "func",
			Arguments: []*Type{{Kind: "any"}, {Kind: "any"}},
			Return:    &Type{Kind: "any"},
		}
	}
	if fn.Validate != nil {
		if t, arity, ok := probeValidate(fn); ok {
			arguments := make([]*Type, arity)
			for i := range arguments {
				arguments[i] = &Type{Kind: "any"}
			}
			return &Type{
				Kind:      "func",
				Arguments: arguments,
				Return:    typeOf(t),
			}
		}
	}
	return &Type{Kind: "func"}
}

var anyInterfaceType = reflect.TypeOf((*any)(nil)).Elem()

// probeValidate calls Validate with interface arguments for arities 0..3 and
// returns the documented return type of the first accepted arity.
func probeValidate(fn *builtin.Function) (reflect.Type, int, bool) {
	for arity := 0; arity <= 3; arity++ {
		args := make([]reflect.Type, arity)
		for i := range args {
			args[i] = anyInterfaceType
		}
		t, err := fn.Validate(args)
		if err == nil {
			return t, arity, true
		}
	}
	return nil, 0, false
}

// typeOf converts a reflect.Type to the documentation Type shape without
// touching the Context appendix: builtins only reference basic kinds,
// arrays, maps, funcs and a few named stdlib types.
func typeOf(t reflect.Type) *Type {
	t = deref.Type(t)

	// Named stdlib types (time.Duration, time.Time, time.Location, ...).
	if t.Name() != "" && t.PkgPath() != "" {
		switch t.Kind() {
		case reflect.Struct:
			return &Type{Kind: "struct", Name: TypeName(t.String())}
		default:
			return &Type{Kind: Kind(t.String())}
		}
	}

	switch t.Kind() {
	case reflect.Bool:
		return &Type{Kind: "bool"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &Type{Kind: "int"}
	case reflect.Float32, reflect.Float64:
		return &Type{Kind: "float"}
	case reflect.String:
		return &Type{Kind: "string"}
	case reflect.Interface:
		return &Type{Kind: "any"}
	case reflect.Slice, reflect.Array:
		return &Type{Kind: "array", Type: typeOf(t.Elem())}
	case reflect.Map:
		return &Type{Kind: "map", Key: typeOf(t.Key()), Type: typeOf(t.Elem())}
	case reflect.Func:
		arguments := make([]*Type, 0, t.NumIn())
		for i := 0; i < t.NumIn(); i++ {
			arguments = append(arguments, typeOf(t.In(i)))
		}
		f := &Type{Kind: "func", Arguments: arguments}
		if t.NumOut() > 0 {
			f.Return = typeOf(t.Out(0))
		}
		return f
	default:
		return &Type{Kind: Kind(t.String())}
	}
}
