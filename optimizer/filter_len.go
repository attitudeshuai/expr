package optimizer

import (
	. "github.com/expr-lang/expr/ast"
)

type filterLen struct {
	p *pipeline
}

func (v *filterLen) Visit(node *Node) {
	if ln, ok := (*node).(*BuiltinNode); ok &&
		ln.Name == "len" &&
		len(ln.Arguments) == 1 {
		if filter, ok := ln.Arguments[0].(*BuiltinNode); ok &&
			filter.Name == "filter" &&
			len(filter.Arguments) == 2 {
			v.p.patchCopyType(node, &BuiltinNode{
				Name:      "count",
				Arguments: filter.Arguments,
			})
		}
	}
}
