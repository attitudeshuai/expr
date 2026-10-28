package optimizer

import (
	"fmt"
	"sort"
	"strings"

	. "github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/conf"
)

// RuleName is the stable name of an optimization rule.
type RuleName string

// Stable optimization rule names. They double as the identifiers used in
// the declarative enabled-rule set and in the attribution report.
const (
	RuleInArray              RuleName = "in_array"
	RuleFold                 RuleName = "fold"
	RuleConstExpr            RuleName = "const_expr"
	RuleInRange              RuleName = "in_range"
	RuleFilterMap            RuleName = "filter_map"
	RuleFilterLen            RuleName = "filter_len"
	RuleFilterLast           RuleName = "filter_last"
	RuleFilterFirst          RuleName = "filter_first"
	RulePredicateCombination RuleName = "predicate_combination"
	RuleSumRange             RuleName = "sum_range"
	RuleSumArray             RuleName = "sum_array"
	RuleSumMap               RuleName = "sum_map"
	RuleCountAny             RuleName = "count_any"
	RuleCountThreshold       RuleName = "count_threshold"
)

// ruleKind says how a rule is executed by the chain.
type ruleKind int

const (
	// ruleOnce walks the tree a single time.
	ruleOnce ruleKind = iota
	// ruleFixpoint walks the tree repeatedly until a walk applies no
	// rewrites, or until the iteration limit is reached.
	ruleFixpoint
)

// Fixpoint iteration limits, carried over verbatim from the former
// hard-coded loops: `for limit := N; limit >= 0; limit--` walks the tree
// at most N+1 times.
const (
	foldIterationLimit      = 1000
	constExprIterationLimit = 100
)

// ruleSpec is the unified descriptor and entry of every optimization rule.
type ruleSpec struct {
	// name is the stable rule name.
	name RuleName
	// kind is ruleOnce or ruleFixpoint.
	kind ruleKind
	// limit is the starting value of the fixpoint loop counter.
	limit int
	// gate reports whether the rule has anything to do. A closed gate
	// skips the rule without walking the tree (rules with a nil gate
	// always run).
	gate func(config *conf.Config) bool
	// makeVisitor builds the rule visitor; fixpoint rules receive a
	// fresh visitor for every round.
	makeVisitor func(p *pipeline) Visitor
}

// rules is the optimization chain, in the exact execution order of the
// former monolithic Optimize implementation. The order must not change.
var rules = []ruleSpec{
	{
		name:        RuleInArray,
		makeVisitor: func(p *pipeline) Visitor { return &inArray{p: p} },
	},
	{
		name:        RuleFold,
		kind:        ruleFixpoint,
		limit:       foldIterationLimit,
		makeVisitor: func(p *pipeline) Visitor { return &fold{p: p} },
	},
	{
		name:  RuleConstExpr,
		kind:  ruleFixpoint,
		limit: constExprIterationLimit,
		gate: func(config *conf.Config) bool {
			return config != nil && len(config.ConstFns) > 0
		},
		makeVisitor: func(p *pipeline) Visitor {
			return &constExpr{p: p, fns: p.config.ConstFns}
		},
	},
	{
		name:        RuleInRange,
		makeVisitor: func(p *pipeline) Visitor { return &inRange{p: p} },
	},
	{
		name:        RuleFilterMap,
		makeVisitor: func(p *pipeline) Visitor { return &filterMap{p: p} },
	},
	{
		name:        RuleFilterLen,
		makeVisitor: func(p *pipeline) Visitor { return &filterLen{p: p} },
	},
	{
		name:        RuleFilterLast,
		makeVisitor: func(p *pipeline) Visitor { return &filterLast{p: p} },
	},
	{
		name:        RuleFilterFirst,
		makeVisitor: func(p *pipeline) Visitor { return &filterFirst{p: p} },
	},
	{
		name:        RulePredicateCombination,
		makeVisitor: func(p *pipeline) Visitor { return &predicateCombination{p: p} },
	},
	{
		name:        RuleSumRange,
		makeVisitor: func(p *pipeline) Visitor { return &sumRange{p: p} },
	},
	{
		name:        RuleSumArray,
		makeVisitor: func(p *pipeline) Visitor { return &sumArray{p: p} },
	},
	{
		name:        RuleSumMap,
		makeVisitor: func(p *pipeline) Visitor { return &sumMap{p: p} },
	},
	{
		name:        RuleCountAny,
		makeVisitor: func(p *pipeline) Visitor { return &countAny{p: p} },
	},
	{
		name:        RuleCountThreshold,
		makeVisitor: func(p *pipeline) Visitor { return &countThreshold{p: p} },
	},
}

// RuleNames returns the names of all optimization rules in chain execution order.
func RuleNames() []string {
	names := make([]string, len(rules))
	for i, spec := range rules {
		names[i] = string(spec.name)
	}
	return names
}

// DefaultRules returns the default enabled rule set (all rules enabled).
// The returned map is a fresh copy and may be modified by the caller.
func DefaultRules() map[string]bool {
	set := make(map[string]bool, len(rules))
	for _, spec := range rules {
		set[string(spec.name)] = true
	}
	return set
}

// AllRulesDisabled reports whether every optimization rule is disabled.
// A nil rule set means the default set and therefore reports false.
func AllRulesDisabled(config *conf.Config) bool {
	if config == nil || config.OptimizeRules == nil {
		return false
	}
	for _, spec := range rules {
		if config.OptimizeRules[string(spec.name)] {
			return false
		}
	}
	return true
}

// Validate checks the declarative optimization configuration before
// compilation: rule names must be taken from the legal name set, and the
// const_expr rule must stay enabled while ConstExpr functions are declared.
func Validate(config *conf.Config) error {
	if config == nil || config.OptimizeRules == nil {
		return nil
	}

	legal := make(map[string]bool, len(rules))
	for _, spec := range rules {
		legal[string(spec.name)] = true
	}

	var unknown []string
	for name := range config.OptimizeRules {
		if !legal[name] {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf(
			"unknown optimization rule %q; valid rules are: %s",
			unknown[0],
			strings.Join(RuleNames(), ", "),
		)
	}

	// Declaring ConstExpr functions while the const_expr rule is disabled
	// would silently drop the declarations, which is an explicit conflict.
	if len(config.ConstFns) > 0 && !config.OptimizeRules[string(RuleConstExpr)] {
		names := make([]string, 0, len(config.ConstFns))
		for name := range config.ConstFns {
			names = append(names, name)
		}
		sort.Strings(names)
		return fmt.Errorf(
			"const expr functions are declared (%s) but the %q optimization rule is disabled; enable the rule or remove the ConstExpr declarations",
			strings.Join(names, ", "),
			RuleConstExpr,
		)
	}

	return nil
}

// ruleEnabled resolves whether a rule is part of the run. A nil rule set
// means the default set, i.e. every rule is enabled.
func ruleEnabled(config *conf.Config, name RuleName) bool {
	if config == nil || config.OptimizeRules == nil {
		return true
	}
	return config.OptimizeRules[string(name)]
}

// fixpointVisitor is implemented by the visitors of fixpoint rules.
type fixpointVisitor interface {
	Visitor
	// changed reports whether the walk performed at least one rewrite.
	changed() bool
	// ruleErr reports the error collected during the walk, if any.
	ruleErr() error
}

// pipeline drives one optimization run over the named-rule chain.
type pipeline struct {
	config *conf.Config
	trace  *Trace
	// rule is the name of the currently running rule.
	rule RuleName
	// round is the current fixpoint round, 1-based; zero for single-pass rules.
	round int
}

// run walks the chain and executes every enabled rule in order.
func (p *pipeline) run(node *Node) error {
	for _, spec := range rules {
		if !ruleEnabled(p.config, spec.name) {
			continue
		}
		if spec.gate != nil && !spec.gate(p.config) {
			p.trace.skip(spec.name, ExitSkipped)
			continue
		}

		p.rule = spec.name
		p.round = 0
		p.trace.start(spec.name)

		if spec.kind == ruleOnce {
			Walk(node, spec.makeVisitor(p))
			p.trace.finish(0, "")
			continue
		}

		rounds := 0
		exitReason := ExitMaxRounds
		for limit := spec.limit; limit >= 0; limit-- {
			rounds++
			p.round = rounds
			visitor := spec.makeVisitor(p)
			Walk(node, visitor)

			fixpoint := visitor.(fixpointVisitor)
			if err := fixpoint.ruleErr(); err != nil {
				p.trace.finish(rounds, ExitError)
				return err
			}
			if !fixpoint.changed() {
				exitReason = ExitConverged
				break
			}
		}
		p.trace.finish(rounds, exitReason)
	}
	return nil
}

// patchWithType records an attributed replacement patch and applies it.
func (p *pipeline) patchWithType(node *Node, newNode Node) {
	p.note(node, newNode)
	patchWithType(node, newNode)
}

// patchCopyType records an attributed copy-type patch and applies it.
func (p *pipeline) patchCopyType(node *Node, newNode Node) {
	p.note(node, newNode)
	patchCopyType(node, newNode)
}

// note records a node replacement against the currently running rule.
// It does nothing when attribution is off.
func (p *pipeline) note(node *Node, next Node) {
	if p.trace == nil {
		return
	}
	location := (*node).Location()
	p.trace.add(Rewrite{
		Round:  p.round,
		From:   location.From,
		To:     location.To,
		Before: shape(*node),
		After:  shape(next),
	})
}

// mutate records an in-place node mutation against the currently running
// rule. It does nothing but run apply when attribution is off.
func (p *pipeline) mutate(node *Node, apply func()) {
	if p.trace == nil {
		apply()
		return
	}
	location := (*node).Location()
	before := shape(*node)
	apply()
	p.trace.add(Rewrite{
		Round:  p.round,
		From:   location.From,
		To:     location.To,
		Before: before,
		After:  shape(*node),
	})
}
