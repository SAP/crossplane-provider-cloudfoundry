package parsan

import (
	"crypto/rand"
	"fmt"
	"maps"
	"math/big"
	"slices"
	"strings"
)

func init() {
	closedCheckedChan = make(chan result)
	close(closedCheckedChan)
}

// closedCheckedChan is a pre-closed channel used for immediate termination during validation
var closedCheckedChan chan result

// namedPathNode represents a single node in the parsing path used to detect recursive cycles
type namedPathNode struct {
	ruleName string // The name of the rule being processed
	input    string // The input string being validated at this node
}

// parseContext maintains the parsing state and tracks the call stack to prevent infinite recursion
type parseContext struct {
	namedPath []namedPathNode // Stack of named rules currently being processed
}

// newParseContext creates a new parsing context with an empty rule stack
func newParseContext() *parseContext {
	return &parseContext{
		namedPath: []namedPathNode{},
	}
}

// hasPathNode checks if a specific named rule with the given input is already being processed,
// which would indicate a recursive cycle
func (pc *parseContext) hasPathNode(node namedPathNode) bool {
	return slices.ContainsFunc(pc.namedPath, func(np namedPathNode) bool {
		return np.input == node.input && np.ruleName == node.ruleName
	})
}

// appendPathNode adds a new rule node to the parsing path stack
func (pc *parseContext) appendPathNode(node namedPathNode) {
	pc.namedPath = append(pc.namedPath, node)
}

// clone creates a deep copy of the parse context to isolate different parsing branches
func (pc *parseContext) clone() *parseContext {
	return &parseContext{
		namedPath: slices.Clone(pc.namedPath),
	}
}

// result represents a validation result containing the accepted portion and remaining input
type result struct {
	sanitized string // The validated/sanitized portion of the input
	toParse   string // The remaining unparsed input
}

// RuleWithMaxLength defines an interface for validation rules that enforce maximum length constraints.
// Implementing types must provide a MaxLength method returning the maximum permissible input length.
//
// During ParseAndSanitize execution, length validation is applied exclusively to the top-level rule
// when it implements this interface. Nested or chained validation rules do not trigger additional
// length checks.
type RuleWithMaxLength interface {
	MaxLength() int
}

// ruleWithLengthConstraint wraps an existing Rule with maximum length validation capability.
// This struct implements both the Rule interface (through embedding) and RuleWithMaxLength.
type ruleWithLengthConstraint struct {
	Rule
	maxLength int
}

// Compile-time check to ensure ruleWithLengthConstraint implements RuleWithMaxLength
var _ RuleWithMaxLength = ruleWithLengthConstraint{}

// RuleWithLengthConstraint wraps an existing rule with maximum length validation.
// It returns a new rule that implements the RuleWithMaxLength interface,
// allowing the validation system to enforce the specified length constraint.
//
// Parameters:
//   - rule: The base validation rule to wrap
//   - length: The maximum allowed length for input validation
//
// Returns a Rule that enforces both the original validation logic and length constraints.
func RuleWithLengthConstraint(rule Rule, length int) Rule {
	return ruleWithLengthConstraint{
		Rule:      rule,
		maxLength: length,
	}
}

// MaxLength returns the maximum allowed length configured for this rule.
// This method satisfies the RuleWithMaxLength interface requirement.
func (r ruleWithLengthConstraint) MaxLength() int {
	return r.maxLength
}

// ParseAndSanitize validates and sanitizes an input string according to the given rule, returning
// a sorted list of valid sanitized suggestions. The input is first truncated to the rule's maximum
// length if applicable, then validated to generate suggestions. Results are sorted by length
// (longest first) and alphabetically for equal lengths.
func ParseAndSanitize(in string, rule Rule) []string {
	if mlRule, ok := rule.(RuleWithMaxLength); ok {
		if len(in) > mlRule.MaxLength() {
			in = in[:mlRule.MaxLength()]
		}
	}
	checked := rule.validate(newParseContext(), in)
	suggestions := map[string]struct{}{}
	for ch := range checked {
		if len(ch.toParse) == 0 {
			if mlRule, ok := rule.(RuleWithMaxLength); ok {
				if mlRule.MaxLength() < len(ch.sanitized) {

					continue
				}
			}
			suggestions[ch.sanitized] = struct{}{}
		}
	}
	return slices.SortedStableFunc(
		maps.Keys(suggestions),
		func(a, b string) int {
			lena := len(a)
			lenb := len(b)
			switch {
			case lena < lenb:
				return 1
			case lena > lenb:
				return -1
			default:
				return strings.Compare(a, b)
			}
		},
	)
}

// Rule interface defines the contract that all ABNF rule types must implement.
// A type implementing Rule can be used to validate and sanitize strings using ParseAndSanitize.
//
// This package provides basic Rule types that can be combined to create more complex validation rules.
type Rule interface {
	validate(*parseContext, string) <-chan result
	// WithSuggestionFunc configures a SuggestionFunc for the rule. When the input string
	// is not valid according to the Rule, the specified function generates valid suggestions
	// that are used for creating sanitized results in ParseAndSanitize.
	WithSuggestionFunc(SuggestionFunc) Rule
}

// terminal represents a rule that matches an exact string literal
type terminal struct {
	s string // The exact string that must be matched
}

var _ Rule = &terminal{}

// Terminal creates a Rule that accepts only the exact string s.
// The WithSuggestionFunc method is not supported for Terminal rules and will panic if called.
func Terminal(s string) Rule {
	return &terminal{
		s: s,
	}
}

// validate checks if the input starts with the terminal's exact string
func (t *terminal) validate(ctx *parseContext, in string) <-chan result {
	out := make(chan result)
	go func() {
		defer close(out)
		if remaining, matches := strings.CutPrefix(in, t.s); matches {
			out <- result{
				sanitized: t.s,
				toParse:   remaining,
			}
		}
	}()
	return out
}

// WithSuggestionFunc is not implemented for Terminal rules and will panic if called
func (t *terminal) WithSuggestionFunc(fn SuggestionFunc) Rule {
	panic("not implemented")
}

// rangeType represents a rule that matches characters within a specified range
type rangeType struct {
	start     rune           // The starting character of the valid range (inclusive)
	end       rune           // The ending character of the valid range (inclusive)
	suggestFn SuggestionFunc // Optional function to generate suggestions for invalid input
}

var _ Rule = &rangeType{}

// Range creates a Rule that accepts a single character within the specified range.
// The range is inclusive of both start and end characters.
//
// For example, Range('a', 'z') creates a rule that accepts any lowercase letter.
//
// The Range rule can be configured with a suggestion function to sanitize invalid input.
func Range(start, end rune) Rule {
	if end < start {
		end = start
	}
	return &rangeType{
		start: start,
		end:   end,
	}
}

// validate checks if the first character of the input falls within the valid range
func (r *rangeType) validate(ctx *parseContext, in string) <-chan result {
	out := make(chan result)
	go func() {
		defer close(out)
		if len(in) > 0 {
			first := rune(in[0])
			if first >= r.start && first <= r.end {
				out <- result{
					sanitized: string(first),
					toParse:   in[1:],
				}
			} else if r.suggestFn != nil {
				for _, checked := range r.suggestFn(in) {
					if checked != nil {
						out <- *checked
					}
				}
			}
		}

	}()
	return out
}

// WithSuggestionFunc configures a suggestion function for this Range rule
func (r *rangeType) WithSuggestionFunc(fn SuggestionFunc) Rule {
	r.suggestFn = fn
	return r
}

// concat represents a rule that matches a sequence of other rules in sequential order
type concat struct {
	type1 Rule // The first rule that must be matched
	type2 Rule // The second rule that must be matched after the first
}

var _ Rule = &concat{}

// Concat creates a Rule that validates input by matching a sequence of rules in order.
// The rule accepts strings where substrings can be validated by the specified rules
// in the given sequence.
//
// For example, Concat(Range('0', '9'), Range('a', 'z')) accepts "5e" but not "AA".
//
// If no rules are provided, Concat creates a rule that accepts empty strings.
// If only one rule is provided, that rule is returned directly.
//
// WithSuggestionFunc is not supported for Concat rules and will panic if called.
func Concat(rules ...Rule) Rule {
	if len(rules) == 0 {
		return (*concat)(nil)
	}
	if len(rules) == 1 {
		return rules[0]
	}
	var merged *concat
	for len(rules) > 1 {
		merged = &concat{
			type1: rules[len(rules)-2],
			type2: rules[len(rules)-1],
		}
		rules = append(rules[:len(rules)-2], merged)
	}
	return merged
}

// validate attempts to match the input against both constituent rules sequentially
func (c *concat) validate(ctx *parseContext, in string) <-chan result {
	out := make(chan result)
	go func() {
		defer close(out)
		if c != nil {
			checkedResults := []result{}
			for checked := range c.type1.validate(ctx.clone(), in) {
				checkedResults = append(checkedResults, checked)
			}
			for _, checkedResult := range checkedResults {
				for checked := range c.type2.validate(ctx.clone(), checkedResult.toParse) {
					out <- result{
						sanitized: checkedResult.sanitized + checked.sanitized,
						toParse:   checked.toParse,
					}
				}
			}
		} else {
			out <- result{
				sanitized: "",
				toParse:   in,
			}
		}
	}()
	return out
}

// WithSuggestionFunc is not implemented for Concat rules and will panic if called
func (c *concat) WithSuggestionFunc(fn SuggestionFunc) Rule {
	panic("not implemented")
}

// alternative represents a rule that succeeds if any of its constituent rules match
type alternative struct {
	types     []Rule         // List of alternative rules to attempt matching
	suggestFn SuggestionFunc // Optional function to generate suggestions when no rules match
}

var _ Rule = &alternative{}

// Alternative creates a Rule that accepts input if any of the provided rules accept it.
// The rule attempts to match against each alternative until one succeeds.
//
// For example, Alternative(Range('a', 'z'), Range('A', 'Z')) accepts both 'e' and 'G'
// but rejects '5'.
func Alternative(types ...Rule) Rule {
	return &alternative{
		types: types,
	}
}

// validate attempts to match the input against each alternative rule
func (a *alternative) validate(ctx *parseContext, in string) <-chan result {
	out := make(chan result)
	go func() {
		defer close(out)
		validated := false
		for _, t := range a.types {
			for checked := range t.validate(ctx.clone(), in) {
				out <- checked
				validated = true
			}
		}
		if a.suggestFn != nil && !validated {
			for _, checked := range a.suggestFn(in) {
				if checked != nil {
					out <- *checked
				}
			}
		}
	}()
	return out
}

// WithSuggestionFunc configures a suggestion function for this Alternative rule
func (a *alternative) WithSuggestionFunc(fn SuggestionFunc) Rule {
	a.suggestFn = fn
	return a
}

// namedTypes is the global registry that maps rule names to their implementations
var namedTypes = map[string]Rule{}

// named represents a reference to a rule identified by its name
type named struct {
	name string // The name of the referenced rule
}

var _ Rule = &named{}

// Named binds a name to the given rule and registers it in the global registry.
// The named rule can be referenced later using RefNamed, enabling lazy evaluation and recursion.
func Named(name string, rule Rule) Rule {
	namedTypes[name] = rule
	return rule
}

// randomNamePrefix is the prefix used for automatically generated rule names
const randomNamePrefix = "___random_name_"

// GetRandomName generates a unique random name that is not currently used by any Named rule.
// This is useful for creating temporary named rules without conflicts.
func GetRandomName() string {
	suffixBigNum, err := rand.Int(rand.Reader, big.NewInt(10000000000))
	if err != nil {
		panic(err)
	}
	suffixNum := suffixBigNum.Int64()
	for i := range 50000 {
		name := fmt.Sprintf("%s%d", randomNamePrefix, i+int(suffixNum))
		if _, ok := namedTypes[name]; !ok {
			return name
		}
	}
	panic("cannot get random name")
}

// RefNamed creates a lazy reference to a named rule. The referenced rule can be defined
// after the reference is created, enabling forward references and recursive rules.
func RefNamed(name string) Rule {
	return &named{
		name: name,
	}
}

// validate looks up the named rule and delegates validation to it, with cycle detection
func (n *named) validate(ctx *parseContext, in string) <-chan result {
	np := namedPathNode{
		ruleName: n.name,
		input:    in,
	}

	if ctx.hasPathNode(np) {
		return closedCheckedChan
	}
	ctx.appendPathNode(np)
	if t, ok := namedTypes[n.name]; ok {
		return t.validate(ctx.clone(), in)
	}
	return closedCheckedChan
}

// WithSuggestionFunc is not implemented for RefNamed rules and will panic if called
func (n *named) WithSuggestionFunc(fn SuggestionFunc) Rule {
	panic("not implemented")
}

// SeqInf represents infinite repetition and can be used as the max parameter in Seq rules
const SeqInf = -1

// Seq creates a Rule that validates strings composed of repeated concatenations of a single rule.
// The repetition count is constrained by the min and max parameters.
//
// Parameters:
//   - min: minimum number of repetitions (treated as 0 if negative)
//   - max: maximum number of repetitions (treated as min if less than min, SeqInf for infinite)
//   - rule: the rule to be repeated
//
// Examples:
//   - Seq(5, 5, Terminal("a")) accepts only "aaaaa"
//   - Seq(1, 3, Terminal("b")) accepts "b", "bb", or "bbb"
//   - Seq(0, 1, rule) accepts empty string or one occurrence of rule
//   - Seq(1, SeqInf, Terminal("c")) accepts "c", "cc", "ccc", etc.
func Seq(min, max int, rule Rule) Rule {
	if min < 0 {
		min = 0
	}
	if max < 0 && max != SeqInf {
		max = SeqInf
	}
	if max != SeqInf && max < min {
		max = min
	}
	if min == max {
		if min == 1 {
			return rule
		}
		return Concat(slices.Repeat[[]Rule]([]Rule{rule}, min)...)
	}
	if max == SeqInf {
		prefix := Seq(min, min, rule)
		ruleName := GetRandomName()
		suffix := Named(ruleName,
			Alternative(
				Concat(),
				rule,
				Concat(rule,
					RefNamed(ruleName)),
			))
		return Concat(prefix, suffix)
	}
	types := make([]Rule, 0, max-min+1)
	for i := min; i <= max; i++ {
		var rType Rule = Concat(slices.Repeat[[]Rule]([]Rule{rule}, i)...)
		types = append(types, rType)
	}
	return Alternative(types...)
}

// Opt creates a Rule that accepts either an empty string or input that matches the given rule.
// This is syntactic sugar for Seq(0, 1, rule).
func Opt(rule Rule) Rule {
	return Seq(0, 1, rule)
}
