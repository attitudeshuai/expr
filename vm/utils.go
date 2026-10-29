package vm

import (
	"reflect"
	"time"

	"github.com/expr-lang/expr/builtin"
)

type (
	Function     = func(params ...any) (any, error)
	SafeFunction = func(params ...any) (any, uint, error)
)

// registeredFunction pairs a compiled program function with its declared
// allocation metric. It is pushed by OpLoadFunc only for functions that
// declare a metric, so OpCallN can account the result; plain Function values
// from the environment keep flowing through OpCallN unchanged.
type registeredFunction struct {
	fn     Function
	memory builtin.MemoryFunc
}

var (
	errorType = reflect.TypeOf((*error)(nil)).Elem()
)

type Scope struct {
	Array reflect.Value
	Index int
	Len   int
	Count int
	Acc   any
	// Fast paths
	Ints    []int
	Floats  []float64
	Strings []string
	Anys    []any
}

// Item returns the current element from the scope using fast paths when available.
func (s *Scope) Item() any {
	if s.Ints != nil {
		return s.Ints[s.Index]
	}
	if s.Floats != nil {
		return s.Floats[s.Index]
	}
	if s.Strings != nil {
		return s.Strings[s.Index]
	}
	if s.Anys != nil {
		return s.Anys[s.Index]
	}
	return s.Array.Index(s.Index).Interface()
}

type groupBy = map[any][]any

type Span struct {
	Name       string  `json:"name"`
	Expression string  `json:"expression"`
	Duration   int64   `json:"duration"`
	Children   []*Span `json:"children"`
	start      time.Time
}

func GetSpan(program *Program) *Span {
	return program.span
}
