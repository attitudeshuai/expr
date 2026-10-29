package builtin

import (
	"reflect"
)

// Category groups builtin functions for documentation purposes.
type Category string

const (
	CategoryString        Category = "String"
	CategoryDate          Category = "Date"
	CategoryNumber        Category = "Number"
	CategoryArray         Category = "Array"
	CategoryMap           Category = "Map"
	CategoryConversion    Category = "Conversion"
	CategoryMiscellaneous Category = "Miscellaneous"
	CategoryBitwise       Category = "Bitwise"
)

// Title returns the human readable category heading used in documentation.
func (c Category) Title() string {
	if c == CategoryConversion {
		return "Type Conversion Functions"
	}
	return string(c) + " Functions"
}

// ArgumentKind describes the shape of a predicate builtin argument.
type ArgumentKind int

const (
	// ArgumentExpression is an ordinary expression argument.
	ArgumentExpression ArgumentKind = iota
	// ArgumentPredicate is a predicate (closure) argument.
	ArgumentPredicate
)

// ArgumentSpec describes one argument slot of a predicate builtin. It is
// the registry-side counterpart of the parser's predicate grammar table.
type ArgumentSpec struct {
	Kind     ArgumentKind
	Optional bool
}

type Function struct {
	Name      string
	Fast      func(arg any) any
	Func      func(args ...any) (any, error)
	Safe      func(args ...any) (any, uint, error)
	Types     []reflect.Type
	Validate  func(args []reflect.Type) (reflect.Type, error)
	Deref     func(i int, arg reflect.Type) bool
	Predicate bool

	// Documentation metadata. The registry is the single source of truth for
	// generated documentation: every builtin must fill all four fields.

	// Category is one of the builtin Category constants.
	Category Category
	// Description is a short, human readable summary (may contain markdown).
	Description string
	// Signature is a human readable call signature, e.g. "trim(str[, chars])".
	Signature string
	// Examples are compilable expression strings (strict empty environment).
	Examples []string
	// Arguments describes argument slots for predicate builtins; nil otherwise.
	Arguments []ArgumentSpec
}

func (f *Function) Type() reflect.Type {
	if len(f.Types) > 0 {
		return f.Types[0]
	}
	return reflect.TypeOf(f.Func)
}
