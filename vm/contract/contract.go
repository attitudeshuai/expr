// Package contract defines the runtime environment contract generated at
// compile time from the environment declaration. The contract is retained by
// the compiled program and serves two purposes:
//
//   - to validate the environment passed at run time before any bytecode is
//     executed, failing fast on missing names and shape mismatches;
//   - to back contract-first gates used by every environment value fetch site
//     of the VM.
//
// The package depends only on the reflect/deref primitives; it must not import
// the nature, builtin or runtime packages to avoid import cycles.
package contract

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/expr-lang/expr/internal/deref"
)

// Kind is the kind of the environment as declared at compile time.
type Kind uint8

const (
	// KindUnknown is used when the environment kind cannot be determined.
	KindUnknown Kind = iota
	// KindStruct is a struct (or pointer to struct) environment.
	KindStruct
	// KindMap is a map environment.
	KindMap
)

// String returns the lowercase name of the environment kind.
func (k Kind) String() string {
	switch k {
	case KindStruct:
		return "struct"
	case KindMap:
		return "map"
	default:
		return "unknown"
	}
}

// Shape describes the type shape of a single environment name. Shapes are
// compiled from the checker natures and contain no runtime values, only
// type-level information.
type Shape struct {
	// Kind is the reflect.Kind of the value, with pointer kinds dereferenced.
	Kind reflect.Kind
	// Type is the dereferenced type signature, used for messages and the
	// contract key.
	Type string
	// Any matches every value (unknown, interface{} or nil declarations).
	Any bool
	// Func marks callable shapes.
	Func bool
	// Elem is the element shape for slices, arrays and maps.
	Elem *Shape
	// Fields are the nested shapes for struct fields and map keys.
	Fields map[string]*Shape
}

// Contract is the environment contract retained by a compiled program.
type Contract struct {
	// Kind is the declared environment kind.
	Kind Kind
	// Strict reports whether the contract was generated in strict mode.
	// In loose mode (AllowUndefinedVariables) undeclared names are simply
	// absent from Names.
	Strict bool
	// FastMap reports whether the declared environment is a map[string]any
	// (served by the OpLoadFast path at run time).
	FastMap bool
	// Names are the effective top-level names (methods, promoted embedded
	// fields and renamed fields included by their effective names).
	Names map[string]*Shape
}

// NameMismatch describes a name whose runtime shape does not match the
// declared one.
type NameMismatch struct {
	Name     string
	Expected string
	Got      string
}

// MismatchError is returned before execution when the runtime environment does
// not satisfy the compiled contract.
type MismatchError struct {
	// Expected and Got describe mismatched environment kinds. Both are empty
	// when only names are missing or mismatched.
	Expected string
	Got      string
	// Missing lists names not resolvable in the runtime environment.
	Missing []string
	// Mismatched lists names whose runtime shape differs from the contract.
	Mismatched []NameMismatch
}

func (e *MismatchError) Error() string {
	var b strings.Builder
	b.WriteString("env contract mismatch")
	if e.Expected != "" {
		fmt.Fprintf(&b, ": expected %s environment, got %s", e.Expected, e.Got)
	}
	if len(e.Missing) > 0 {
		b.WriteString("; missing names: ")
		b.WriteString(strings.Join(e.Missing, ", "))
	}
	if len(e.Mismatched) > 0 {
		b.WriteString("; mismatched names: ")
		for i, m := range e.Mismatched {
			if i > 0 {
				b.WriteString(", ")
			}
			if m.Name != "" {
				fmt.Fprintf(&b, "%s ", m.Name)
			}
			fmt.Fprintf(&b, "(expected %s, got %s)", m.Expected, m.Got)
		}
	}
	return b.String()
}

// Validate checks the runtime environment against the contract. It returns
// nil when the environment satisfies all declared names. Extra names are
// always allowed; nested map keys are not validated up front, as documented
// map semantics treat missing nested keys as nil.
func (c *Contract) Validate(env any) error {
	if c == nil {
		return nil
	}
	v := reflect.ValueOf(env)
	if !v.IsValid() {
		return &MismatchError{Expected: c.Kind.String(), Got: "nil"}
	}
	got := valueKind(v)
	if got != c.Kind {
		return &MismatchError{Expected: c.Kind.String(), Got: runtimeKind(v)}
	}

	var missing []string
	var mismatched []NameMismatch
	for name, shape := range c.Names {
		resolved, ok := c.resolve(v, name)
		if !ok {
			missing = append(missing, name)
			continue
		}
		if msg, ok := compare(shape, resolved); !ok {
			mismatched = append(mismatched, NameMismatch{
				Name:     name,
				Expected: msg.expected,
				Got:      msg.got,
			})
		}
	}
	if len(missing) == 0 && len(mismatched) == 0 {
		return nil
	}
	sort.Strings(missing)
	sort.Slice(mismatched, func(i, j int) bool { return mismatched[i].Name < mismatched[j].Name })
	return &MismatchError{Missing: missing, Mismatched: mismatched}
}

// resolvedValue carries the dereferenced type of a resolved name.
type resolvedValue struct {
	typ reflect.Type
}

// resolve mirrors the effective-name resolution of vm/runtime: methods of the
// original (possibly pointer) value are tried first, then exported fields
// (including promoted embedded fields and expr-tag renames) of the
// dereferenced type.
func (c *Contract) resolve(v reflect.Value, name string) (resolvedValue, bool) {
	if method := v.MethodByName(name); method.IsValid() {
		return resolvedValue{typ: method.Type()}, true
	}
	switch c.Kind {
	case KindStruct:
		t := deref.Type(v.Type())
		if t == nil || t.Kind() != reflect.Struct {
			return resolvedValue{}, false
		}
		field, ok := t.FieldByNameFunc(func(fieldName string) bool {
			sf, _ := t.FieldByName(fieldName)
			return tagMatches(sf, name, fieldName)
		})
		if ok && field.IsExported() {
			return resolvedValue{typ: field.Type}, true
		}
	case KindMap:
		dv := deref.Value(v)
		if dv.Kind() != reflect.Map {
			return resolvedValue{}, false
		}
		if dv.Type().Key().Kind() != reflect.String {
			// Only string-keyed maps can carry the contracted names.
			return resolvedValue{}, false
		}
		elem := dv.MapIndex(reflect.ValueOf(name))
		if !elem.IsValid() {
			return resolvedValue{}, false
		}
		if elem.Kind() == reflect.Interface {
			if elem.IsNil() {
				return resolvedValue{typ: nil}, true
			}
			return resolvedValue{typ: elem.Elem().Type()}, true
		}
		return resolvedValue{typ: elem.Type()}, true
	}
	return resolvedValue{}, false
}

// tagMatches mirrors runtime.findStructField's expr-tag predicate.
func tagMatches(sf reflect.StructField, wanted, fieldName string) bool {
	switch sf.Tag.Get("expr") {
	case "-":
		return false
	case wanted:
		return true
	default:
		return fieldName == wanted
	}
}

type mismatchMessage struct {
	expected, got string
}

// compare returns a mismatch message when the resolved type does not satisfy
// the declared shape. Unknown/interface/nil shapes match everything.
func compare(shape *Shape, got resolvedValue) (mismatchMessage, bool) {
	if shape == nil || shape.Any {
		return mismatchMessage{}, true
	}
	if got.typ == nil {
		return mismatchMessage{expected: shape.label(), got: "nil"}, false
	}
	k := deref.Type(got.typ).Kind()
	if shape.Func {
		if k == reflect.Func {
			return mismatchMessage{}, true
		}
	} else if k == shape.Kind {
		return mismatchMessage{}, true
	}
	return mismatchMessage{expected: shape.label(), got: kindLabel(got.typ, k)}, false
}

func (s *Shape) label() string {
	if s == nil {
		return "any"
	}
	if s.Any {
		return "any"
	}
	if s.Func {
		return "func"
	}
	return s.Kind.String()
}

func kindLabel(t reflect.Type, k reflect.Kind) string {
	if t == nil {
		return "nil"
	}
	return k.String()
}

func valueKind(v reflect.Value) Kind {
	k := deref.Value(v).Kind()
	switch k {
	case reflect.Struct:
		return KindStruct
	case reflect.Map:
		return KindMap
	default:
		return KindUnknown
	}
}

// runtimeKind renders the dereferenced reflect kind for error messages.
func runtimeKind(v reflect.Value) string {
	if !v.IsValid() {
		return "nil"
	}
	return deref.Value(v).Kind().String()
}

// Key returns a stable fingerprint of the contract suitable for participating
// in artifact cache keys. Two contracts with the same kind, strictness, fast
// map flag and name shapes produce the same key.
func (c *Contract) Key() string {
	var b strings.Builder
	fmt.Fprintf(&b, "env:%s:strict=%t:fast=%t", c.Kind, c.Strict, c.FastMap)
	names := make([]string, 0, len(c.Names))
	for name := range c.Names {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		b.WriteByte('|')
		b.WriteString(name)
		b.WriteByte('=')
		writeShapeKey(&b, c.Names[name])
	}
	return b.String()
}

func writeShapeKey(b *strings.Builder, s *Shape) {
	if s == nil || s.Any {
		b.WriteString("any")
		return
	}
	if s.Func {
		b.WriteString("func:")
		b.WriteString(s.Type)
		return
	}
	b.WriteString(s.Kind.String())
	if s.Type != "" {
		b.WriteByte(':')
		b.WriteString(s.Type)
	}
	if s.Elem != nil {
		b.WriteString("[]")
		writeShapeKey(b, s.Elem)
	}
	if len(s.Fields) > 0 {
		names := make([]string, 0, len(s.Fields))
		for name := range s.Fields {
			names = append(names, name)
		}
		sort.Strings(names)
		b.WriteByte('{')
		for i, name := range names {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(name)
			b.WriteByte(':')
			writeShapeKey(b, s.Fields[name])
		}
		b.WriteByte('}')
	}
}

// Expectation is the contract shape attached to a single dynamic fetch
// instruction, keyed by its bytecode position.
type Expectation struct {
	// Receiver is the expected shape of the fetched value.
	Receiver *Shape
	// Result is the expected shape of the produced value. nil results (for
	// example missing map keys) always pass.
	Result *Shape
}

// Guard is a per-run contract gate. All fetch sites of the VM pass through it
// before touching the runtime fetch functions. A nil *Guard is a no-op.
type Guard struct {
	contract *Contract
	expects  map[int]*Expectation
}

// NewGuard returns a Guard for the contract and per-position expectations. It
// returns nil when contract is nil, which makes every gate a single nil check
// on the dormant path.
func NewGuard(c *Contract, expects map[int]*Expectation) *Guard {
	if c == nil {
		return nil
	}
	return &Guard{contract: c, expects: expects}
}

func (g *Guard) expectation(ip int) *Expectation {
	if g == nil || g.expects == nil {
		return nil
	}
	return g.expects[ip]
}

// Root gates a top-level name load. Names not part of the contract (possible
// in loose mode only) pass through unchanged.
func (g *Guard) Root(env any, name string) {
	if g == nil {
		return
	}
	shape, ok := g.contract.Names[name]
	if !ok {
		return
	}
	v := reflect.ValueOf(env)
	if !v.IsValid() {
		g.fail([]string{name}, nil)
	}
	if _, ok := g.contract.resolve(v, name); !ok {
		g.fail([]string{name}, nil)
		return
	}
	if msg, ok := compare(shape, g.resolveValue(v, name)); !ok {
		g.fail(nil, []NameMismatch{{Name: name, Expected: msg.expected, Got: msg.got}})
	}
}

func (g *Guard) resolveValue(v reflect.Value, name string) resolvedValue {
	r, _ := g.contract.resolve(v, name)
	return r
}

// Field gates a struct field fetch. It verifies the effective field path is
// resolvable by name on the actual receiver, which also prevents a field
// index compiled for one struct from silently reading a wrong field of
// another struct.
func (g *Guard) Field(v any, want *Shape, path []string) {
	if g == nil || v == nil || len(path) == 0 {
		return
	}
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return
	}
	if valueKind(rv) != KindStruct {
		g.fail(nil, []NameMismatch{{Name: strings.Join(path, "."), Expected: "struct", Got: kindLabel(rv.Type(), deref.Value(rv).Kind())}})
		return
	}
	t := rv.Type()
	for i, name := range path {
		field, ok := deref.Type(t).FieldByNameFunc(func(fieldName string) bool {
			sf, _ := deref.Type(t).FieldByName(fieldName)
			return tagMatches(sf, name, fieldName)
		})
		if !ok || !field.IsExported() {
			g.fail([]string{strings.Join(path, ".")}, nil)
			return
		}
		t = field.Type
		if i == len(path)-1 {
			if msg, ok := compare(want, resolvedValue{typ: field.Type}); !ok {
				g.fail(nil, []NameMismatch{{Name: strings.Join(path, "."), Expected: msg.expected, Got: msg.got}})
			}
		}
	}
}

// Method gates a method fetch. Besides the method name it verifies the
// compiled method index still resolves to the same name on the actual type,
// which prevents silently invoking a different method when a program
// compiled against one struct is run against another unrelated struct that
// happens to declare a method with the same name.
func (g *Guard) Method(v any, idx int, name string) {
	if g == nil || v == nil || name == "" {
		return
	}
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return
	}
	// Interface-typed values expose their concrete method set by name only.
	if rv.Kind() == reflect.Interface {
		if rv.MethodByName(name).IsValid() {
			return
		}
		g.fail([]string{name}, nil)
	}
	// For every concrete type, the compiled index must still point at a
	// method with the same name.
	if t := rv.Type(); idx >= 0 && idx < t.NumMethod() && t.Method(idx).Name == name {
		return
	}
	g.fail([]string{name}, nil)
}

// Receiver gates the receiver of a dynamic fetch (OpFetch), slice or
// fetch-builtin before the value is read.
func (g *Guard) Receiver(ip int, v any) {
	if g == nil || v == nil {
		return
	}
	e := g.expectation(ip)
	if e == nil || e.Receiver == nil || e.Receiver.Any {
		return
	}
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return
	}
	got := deref.Value(rv).Kind()
	if got != e.Receiver.Kind {
		g.fail(nil, []NameMismatch{{Expected: e.Receiver.label(), Got: got.String()}})
	}
}

// Result gates the result of a dynamic fetch (OpFetch). nil results always
// pass to preserve the documented "missing map key is nil" semantics; zero
// values still match their declared element kind.
func (g *Guard) Result(ip int, v any) {
	if g == nil || v == nil {
		return
	}
	e := g.expectation(ip)
	if e == nil || e.Result == nil || e.Result.Any {
		return
	}
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return
	}
	got := deref.Value(rv).Kind()
	if got != e.Result.Kind {
		g.fail(nil, []NameMismatch{{Expected: e.Result.label(), Got: got.String()}})
	}
}

func (g *Guard) fail(missing []string, mismatched []NameMismatch) {
	sort.Strings(missing)
	sort.Slice(mismatched, func(i, j int) bool { return mismatched[i].Name < mismatched[j].Name })
	panic(&MismatchError{Missing: missing, Mismatched: mismatched})
}
