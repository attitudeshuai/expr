package conf

import (
	"reflect"

	"github.com/expr-lang/expr/checker/nature"
	"github.com/expr-lang/expr/internal/deref"
	"github.com/expr-lang/expr/vm/contract"
)

// NewEnvContract builds the runtime environment contract from the declared
// environment nature. It is invoked after all visitors have run, so names
// injected by patchers (for example WithContext) participate in the contract.
// It returns nil when the nature does not describe a struct or map
// environment.
//
// The contract contains effective names only: methods, promoted embedded
// fields and expr-tag-renamed fields are included under their effective
// names; fields tagged with `expr:"-"` are excluded by the nature itself.
func NewEnvContract(env nature.Nature, cache *nature.Cache) *contract.Contract {
	_, k, _ := deref.TypeKind(env.Type, env.Kind)
	var kind contract.Kind
	switch k {
	case reflect.Struct:
		kind = contract.KindStruct
	case reflect.Map:
		kind = contract.KindMap
	default:
		return nil
	}

	all := env.All(cache)
	names := make(map[string]*contract.Shape, len(all))
	visiting := make(map[reflect.Type]bool)
	for name, nt := range all {
		names[name] = shapeOf(nt, cache, visiting)
	}

	return &contract.Contract{
		Kind:    kind,
		Strict:  env.Strict,
		FastMap: env.IsFastMap(),
		Names:   names,
	}
}

// NewShape converts a single checker nature into a runtime contract shape.
func NewShape(n nature.Nature, cache *nature.Cache) *contract.Shape {
	return shapeOf(n, cache, make(map[reflect.Type]bool))
}

// shapeOf converts a nature to a Shape, guarding against self-referential
// types by truncating nested field expansion of types currently being
// visited.
func shapeOf(n nature.Nature, cache *nature.Cache, visiting map[reflect.Type]bool) *contract.Shape {
	s := &contract.Shape{}

	// Declared nil values and unknown/interface types match anything.
	if n.Nil {
		s.Any = true
		return s
	}
	if n.Type == nil || n.IsUnknown(cache) {
		s.Any = true
		return s
	}

	t, k, _ := deref.TypeKind(n.Type, n.Kind)
	s.Kind = k
	s.Type = t.String()

	switch k {
	case reflect.Func:
		s.Func = true

	case reflect.Struct:
		if !visiting[t] {
			visiting[t] = true
			nt := cache.FromType(t)
			for name, fn := range nt.All(cache) {
				if s.Fields == nil {
					s.Fields = make(map[string]*contract.Shape)
				}
				s.Fields[name] = shapeOf(fn, cache, visiting)
			}
			visiting[t] = false
		}

	case reflect.Map:
		if n.TypeData != nil {
			for name, fn := range n.Fields {
				if s.Fields == nil {
					s.Fields = make(map[string]*contract.Shape)
				}
				s.Fields[name] = shapeOf(fn, cache, visiting)
			}
		}
		if elem := n.Elem(cache); elem.Type != nil || elem.Nil {
			s.Elem = shapeOf(elem, cache, visiting)
		}

	case reflect.Slice, reflect.Array:
		if elem := n.Elem(cache); elem.Type != nil || elem.Nil {
			s.Elem = shapeOf(elem, cache, visiting)
		}
	}

	return s
}
