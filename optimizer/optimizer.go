package optimizer

import (
	"fmt"
	"reflect"

	. "github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/conf"
)

// Optimize runs the named-rule optimization chain over the tree.
//
// The executed rule set is taken from config.OptimizeRules; a nil set means
// the default set of all rules. Rule names are validated up front, unknown
// names return an error. When config.OptimizeTrace holds a *Trace, every
// node rewrite is attributed to its rule in the trace's report; otherwise
// no attribution data is collected.
//
// The signature is kept stable so existing callers need no changes: with a
// nil config or a config without the new fields the behavior is identical
// to the former monolithic implementation.
func Optimize(node *Node, config *conf.Config) error {
	if err := Validate(config); err != nil {
		return err
	}

	var trace *Trace
	if config != nil {
		trace, _ = config.OptimizeTrace.(*Trace)
	}
	trace.reset()

	p := &pipeline{
		config: config,
		trace:  trace,
	}
	return p.run(node)
}

var (
	boolType    = reflect.TypeOf(true)
	integerType = reflect.TypeOf(0)
	floatType   = reflect.TypeOf(float64(0))
	stringType  = reflect.TypeOf("")
)

func patchWithType(node *Node, newNode Node) {
	switch n := newNode.(type) {
	case *BoolNode:
		newNode.SetType(boolType)
	case *IntegerNode:
		newNode.SetType(integerType)
	case *FloatNode:
		newNode.SetType(floatType)
	case *StringNode:
		newNode.SetType(stringType)
	case *ConstantNode:
		newNode.SetType(reflect.TypeOf(n.Value))
	case *BinaryNode:
		newNode.SetType(n.Type())
	default:
		panic(fmt.Sprintf("unknown type %T", newNode))
	}
	Patch(node, newNode)
}

func patchCopyType(node *Node, newNode Node) {
	t := (*node).Type()
	newNode.SetType(t)
	Patch(node, newNode)
}
