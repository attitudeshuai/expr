package optimizer

import (
	"fmt"
	"reflect"

	. "github.com/expr-lang/expr/ast"
)

// Exit reasons recorded for rules that run to a fixpoint.
const (
	// ExitConverged means the last tree walk performed no rewrites.
	ExitConverged = "converged"
	// ExitMaxRounds means the iteration limit was reached while rewrites
	// were still being applied.
	ExitMaxRounds = "max_rounds"
	// ExitSkipped means the rule was enabled but had nothing to do
	// (for example const_expr without ConstExpr functions).
	ExitSkipped = "skipped"
	// ExitError means a rewrite reported an error and optimization aborted.
	ExitError = "error"
)

// Trace collects per-rule rewrite attribution for a single optimization run.
// A trace is attached to the configuration via expr.WithOptimizationTrace.
// When no trace is attached, the optimization chain performs no collection
// work at all.
//
// The collected report contains only stable, value-free information:
// source positions and short structural shape descriptions. It never
// contains pointers or node values, so compiling the same expression twice
// produces identical reports.
type Trace struct {
	report *Report
}

// NewTrace creates an empty optimization trace.
func NewTrace() *Trace {
	return &Trace{report: &Report{}}
}

// Report returns the attribution report of the last optimization run.
// It returns nil for a nil trace.
func (t *Trace) Report() *Report {
	if t == nil {
		return nil
	}
	return t.report
}

// reset starts a new report. It is called by the optimization chain.
func (t *Trace) reset() {
	if t == nil {
		return
	}
	t.report = &Report{}
}

// start records that an enabled rule is about to run.
func (t *Trace) start(name RuleName) {
	if t == nil {
		return
	}
	t.report.Rules = append(t.report.Rules, RuleReport{Name: string(name)})
}

// skip records that an enabled rule did not run because its gate was closed.
func (t *Trace) skip(name RuleName, reason string) {
	if t == nil {
		return
	}
	t.report.Rules = append(t.report.Rules, RuleReport{
		Name:       string(name),
		ExitReason: reason,
	})
}

// finish fills in loop bookkeeping of the current rule report.
func (t *Trace) finish(rounds int, reason string) {
	if t == nil {
		return
	}
	rr := &t.report.Rules[len(t.report.Rules)-1]
	rr.Rounds = rounds
	rr.ExitReason = reason
}

// add attributes a single node rewrite to the currently running rule.
func (t *Trace) add(rewrite Rewrite) {
	if t == nil {
		return
	}
	rr := &t.report.Rules[len(t.report.Rules)-1]
	rewrite.Rule = rr.Name
	rr.Rewrites = append(rr.Rewrites, rewrite)
	rr.Hits++
}

// Report is the optimization attribution of one compilation.
type Report struct {
	// Rules holds one entry per rule that was enabled for the run,
	// in chain execution order.
	Rules []RuleReport
}

// Rule returns the report of the rule with the given name.
func (r *Report) Rule(name string) (RuleReport, bool) {
	if r == nil {
		return RuleReport{}, false
	}
	for i := range r.Rules {
		if r.Rules[i].Name == name {
			return r.Rules[i], true
		}
	}
	return RuleReport{}, false
}

// RuleReport attributes rewrites of a single rule.
type RuleReport struct {
	// Name is the stable rule name.
	Name string
	// Hits is the number of node rewrites attributed to the rule.
	Hits int
	// Rounds is the number of tree walks for fixpoint rules.
	// It is zero for single-pass rules.
	Rounds int
	// ExitReason is the fixpoint exit reason (ExitConverged,
	// ExitMaxRounds, ExitSkipped or ExitError). It is empty for
	// single-pass rules.
	ExitReason string
	// Rewrites is the ordered list of attributed node rewrites.
	Rewrites []Rewrite
}

// Rewrite describes a single attributed node rewrite.
type Rewrite struct {
	// Rule is the name of the rule that performed the rewrite.
	Rule string
	// Round is the tree walk number for fixpoint rules (1-based),
	// zero for single-pass rules.
	Round int
	// From and To are the source byte offsets of the rewritten node.
	From int
	To   int
	// Before and After are short shape descriptions of the rewritten
	// node before and after the rewrite, without any values or pointers.
	Before string
	After  string
}

// shape returns a short, stable structural description of a node.
// It never renders node values or addresses, so its output does not
// depend on allocation and is identical across repeated compilations.
func shape(node Node) string {
	switch n := node.(type) {
	case *NilNode:
		return "nil"
	case *IdentifierNode:
		return "identifier(" + n.Value + ")"
	case *IntegerNode:
		return "integer"
	case *FloatNode:
		return "float"
	case *BoolNode:
		return "bool"
	case *StringNode:
		return "string"
	case *BytesNode:
		return "bytes"
	case *ConstantNode:
		if n.Value == nil {
			return "constant(nil)"
		}
		return "constant(" + reflect.TypeOf(n.Value).String() + ")"
	case *UnaryNode:
		return "unary(" + n.Operator + ")"
	case *BinaryNode:
		return "binary(" + n.Operator + ")"
	case *ChainNode:
		return "chain"
	case *MemberNode:
		return "member"
	case *SliceNode:
		return "slice"
	case *CallNode:
		return "call"
	case *BuiltinNode:
		s := "builtin(" + n.Name + ")"
		if n.Threshold != nil {
			s += fmt.Sprintf("{threshold=%d}", *n.Threshold)
		}
		return s
	case *PredicateNode:
		return "predicate"
	case *PointerNode:
		if n.Name == "" {
			return "pointer"
		}
		return "pointer(" + n.Name + ")"
	case *ConditionalNode:
		return "conditional"
	case *VariableDeclaratorNode:
		return "let(" + n.Name + ")"
	case *SequenceNode:
		return fmt.Sprintf("sequence[%d]", len(n.Nodes))
	case *ArrayNode:
		return fmt.Sprintf("array[%d]", len(n.Nodes))
	case *MapNode:
		return fmt.Sprintf("map[%d]", len(n.Pairs))
	case *PairNode:
		return "pair"
	default:
		return "unknown"
	}
}
