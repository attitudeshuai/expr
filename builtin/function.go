package builtin

import (
	"reflect"
)

// MemoryFunc reports the accounted allocation size of a function call, given
// the call arguments and its result. Accounting units are the same abstract
// units used elsewhere by the VM (element/character counts, not bytes).
// A nil metric on a Function makes the VM apply DefaultMemoryFunc.
type MemoryFunc func(args []any, result any) uint

type Function struct {
	Name      string
	Fast      func(arg any) any
	Func      func(args ...any) (any, error)
	Safe      func(args ...any) (any, uint, error)
	Memory    MemoryFunc
	Types     []reflect.Type
	Validate  func(args []reflect.Type) (reflect.Type, error)
	Deref     func(i int, arg reflect.Type) bool
	Predicate bool
}

// DefaultMemoryFunc is the default allocation metric applied to Fast and Func
// implementations that do not declare a custom Memory metric. It measures the
// call result: strings contribute their length, arrays, slices and maps their
// element count, and any other result contributes zero. Custom metrics may
// wrap DefaultMemoryFunc to account for allocations not visible in the result.
func DefaultMemoryFunc(_ []any, result any) uint {
	switch v := reflect.ValueOf(result); v.Kind() {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
		return uint(v.Len())
	default:
		return 0
	}
}

func (f *Function) Type() reflect.Type {
	if len(f.Types) > 0 {
		return f.Types[0]
	}
	return reflect.TypeOf(f.Func)
}
