package parsan

import (
	"crypto/rand"
	"fmt"
	"maps"
	"math/big"
	"slices"
	"strings"
	"sync"
)

func init() {
	closedCheckedChan = make(chan result)
	close(closedCheckedChan)
}

// closedCheckedChan is a pre-closed channel that returns immediately when read.
// It is used to signal early termination in validation routines, particularly
// when cycle detection determines that further processing would result in infinite recursion.
var closedCheckedChan chan result

// namedPathNode represents a single entry in the parsing call stack.
// It captures both the rule name and the input string at that point,
// enabling detection of recursive cycles where the same rule is invoked
// with identical input, which would lead to infinite recursion.
type namedPathNode struct {
	ruleName string // The name of the rule being processed
	input    string // The input string being validated at this point in the call stack
}

// parseContext maintains the state during recursive parsing operations.
// It tracks the call stack of named rules to detect and prevent infinite
// recursion that could occur with self-referential grammar definitions.
type parseContext struct {
	namedPath []namedPathNode // Stack of named rules with their inputs, used for cycle detection
}

// newParseContext creates and returns a new parseContext with an empty call stack.
// This should be called at the beginning of each top-level validation operation.
func newParseContext() *parseContext {
	return &parseContext{
		namedPath: []namedPathNode{},
	}
}

// hasPathNode checks whether the given namedPathNode already exists in the current call stack.
// Returns true if an identical combination of rule name and input string is found,
// indicating that the parser has entered a recursive cycle and should terminate
// to prevent infinite recursion.
func (pc *parseContext) hasPathNode(node namedPathNode) bool {
	return slices.ContainsFunc(pc.namedPath, func(np namedPathNode) bool {
		return np.input == node.input && np.ruleName == node.ruleName
	})
}

// appendPathNode pushes a new namedPathNode onto the call stack.
// This should be called when entering a named rule during validation
// to enable subsequent cycle detection.
func (pc *parseContext) appendPathNode(node namedPathNode) {
	pc.namedPath = append(pc.namedPath, node)
}

// clone creates and returns a deep copy of the parseContext.
// This is necessary when exploring multiple parsing branches in parallel,
// as each branch needs its own independent copy of the call stack
// to correctly detect cycles within that specific branch.
func (pc *parseContext) clone() *parseContext {
	return &parseContext{
		namedPath: slices.Clone(pc.namedPath),
	}
}

// result represents a single parsing outcome containing the successfully
// validated portion of the input and the remaining unparsed suffix.
// Multiple results may be produced when a rule has ambiguous matches.
type result struct {
	sanitized string // The portion of input that was successfully validated and potentially transformed
	toParse   string // The remaining input that has not yet been processed
}

// RuleWithMaxLength is an optional interface that rules can implement to specify
// a maximum allowed length for input strings.
//
// When a rule implements this interface, ParseAndSanitize will:
//   - Truncate input strings exceeding the maximum length before validation
//   - Filter out any sanitized results that exceed the maximum length
//
// Note: Length validation is applied only to the top-level rule passed to
// ParseAndSanitize. Nested rules implementing this interface are not checked
// for length constraints during recursive validation.
type RuleWithMaxLength interface {
	MaxLength() int
}

// ruleWithLengthConstraint is a decorator that wraps an existing Rule
// and adds maximum length validation capability by implementing RuleWithMaxLength.
// The underlying rule's validation logic is preserved through embedding.
type ruleWithLengthConstraint struct {
	Rule          // Embedded rule providing the core validation logic
	maxLength int // Maximum allowed length
}

// Compile-time verification that ruleWithLengthConstraint satisfies RuleWithMaxLength
var _ RuleWithMaxLength = ruleWithLengthConstraint{}

// RuleWithLengthConstraint wraps an existing rule with a maximum length constraint.
// The returned rule implements RuleWithMaxLength, causing ParseAndSanitize to:
//   - Truncate input exceeding the specified length before validation
//   - Exclude sanitized results that exceed the specified length
//
// Parameters:
//   - rule: The base validation rule to wrap
//   - length: The maximum allowed length
//
// Returns a new Rule that enforces both the original validation logic
// and the specified length constraint.
func RuleWithLengthConstraint(rule Rule, length int) Rule {
	return ruleWithLengthConstraint{
		Rule:      rule,
		maxLength: length,
	}
}

// MaxLength returns the maximum allowed length configured for this rule.
// Implements the RuleWithMaxLength interface.
func (r ruleWithLengthConstraint) MaxLength() int {
	return r.maxLength
}

// ParseAndSanitize validates an input string against the provided rule and returns
// all valid sanitized interpretations as a sorted slice of strings.
//
// The function performs the following operations:
//  1. If the rule implements RuleWithMaxLength, truncates input to the maximum length
//  2. Validates the input against the rule, collecting all possible interpretations
//  3. Filters results to include only those that fully consume the input
//  4. If length-constrained, excludes results exceeding the maximum length
//  5. Deduplicates and sorts results by length (longest first), then alphabetically
//
// Parameters:
//   - in: The input string to validate and sanitize
//   - rule: The validation rule defining acceptable input patterns
//
// Returns a slice of unique sanitized strings sorted by descending length,
// with alphabetical ordering as a tiebreaker. Returns an empty slice if
// no valid interpretations exist.
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

// Rule is the core interface that all validation rule types must implement.
// Rules define patterns for validating and sanitizing input strings, and can
// be composed together to create complex grammar definitions.
//
// The validate method is internal to the package and performs the actual
// validation logic, returning results through a channel to support streaming
// of multiple possible interpretations.
//
// Built-in rule types include:
//   - Terminal: Matches exact string literals
//   - Range: Matches single characters within a specified range
//   - Concat: Matches sequences of rules in order
//   - Alternative: Matches any one of several possible rules
//   - Named/RefNamed: Enables recursive and forward-referenced rules
//   - Seq: Matches repeated occurrences of a rule
//   - Opt: Matches zero or one occurrence of a rule
type Rule interface {
	// validate performs validation of the input string against this rule.
	// It returns a channel that yields all possible parsing results.
	// The parseContext tracks the call stack for cycle detection.
	validate(*parseContext, string) <-chan result

	// WithSuggestionFunc attaches a SuggestionFunc to this rule.
	// When the input fails validation, the suggestion function is invoked
	// to generate alternative valid results that can be used for sanitization.
	// Not all rule types support this method; unsupported types will panic.
	WithSuggestionFunc(SuggestionFunc) Rule
}

// terminal is a rule that matches an exact string literal.
// It succeeds only when the input begins with the specified string,
// consuming exactly that portion of the input.
type terminal struct {
	s string // The exact string literal that must appear at the start of the input
}

var _ Rule = &terminal{}

// Terminal creates a Rule that matches the exact string s at the beginning of the input.
// The rule succeeds if and only if the input starts with s, consuming exactly
// len(s) characters and leaving the remainder for subsequent rules.
//
// Example:
//
//	Terminal("hello") matches "hello world" producing sanitized="hello", remaining=" world"
//
// Note: WithSuggestionFunc is not supported for Terminal rules and will panic if called,
// as the terminal string itself serves as the only valid suggestion.
func Terminal(s string) Rule {
	return &terminal{
		s: s,
	}
}

// validate checks if the input begins with this terminal's string literal.
// If matched, it yields a single result with the terminal string as sanitized output
// and the remainder of the input as toParse. If not matched, yields no results.
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

// WithSuggestionFunc panics when called on a Terminal rule.
// Terminal rules do not support suggestion functions because the terminal
// string itself is the only valid match.
func (t *terminal) WithSuggestionFunc(fn SuggestionFunc) Rule {
	panic("not implemented")
}

// rangeType is a rule that matches a single character within an inclusive range.
// It succeeds when the first character of the input falls between start and end (inclusive),
// consuming exactly one character.
type rangeType struct {
	start     rune           // The lower bound of the valid character range (inclusive)
	end       rune           // The upper bound of the valid character range (inclusive)
	suggestFn SuggestionFunc // Optional function to generate suggestions when input doesn't match
}

var _ Rule = &rangeType{}

// Range creates a Rule that matches a single character within the inclusive range [start, end].
// The rule succeeds if the first character of the input is greater than or equal to start
// and less than or equal to end, consuming exactly one character.
//
// If end is less than start, end is automatically set equal to start, creating a rule
// that matches only the single character start.
//
// Examples:
//
//	Range('a', 'z') matches any lowercase ASCII letter
//	Range('0', '9') matches any ASCII digit
//	Range('A', 'Z') matches any uppercase ASCII letter
//
// The rule can be configured with WithSuggestionFunc to provide alternative valid
// characters when the input doesn't match the range.
func Range(start, end rune) Rule {
	if end < start {
		end = start
	}
	return &rangeType{
		start: start,
		end:   end,
	}
}

// validate checks if the first character of the input falls within [start, end].
// If matched, yields a result with that character as sanitized output.
// If not matched and a suggestion function is configured, invokes it to generate
// alternative valid results.
func (r *rangeType) validate(ctx *parseContext, in string) <-chan result {
	out := make(chan result)
	suggested := false
	go func() {
		defer close(out)
		if len(in) > 0 {
			first := rune(in[0])
			if first >= r.start && first <= r.end {
				suggested = true
				out <- result{
					sanitized: string(first),
					toParse:   in[1:],
				}
			}
		}
		if !suggested && r.suggestFn != nil {
			for _, checked := range r.suggestFn(in) {
				if checked != nil {
					out <- *checked
				}
			}
		}
	}()
	return out
}

// WithSuggestionFunc attaches a suggestion function to this Range rule.
// The function will be called when input doesn't match the character range,
// allowing generation of valid alternative characters for sanitization.
// Returns the modified rule to enable method chaining.
func (r *rangeType) WithSuggestionFunc(fn SuggestionFunc) Rule {
	r.suggestFn = fn
	return r
}

// concat is a rule that matches a sequence of two rules in order.
// It succeeds when the input can be split such that the first portion
// matches type1 and the remaining portion matches type2.
// Multiple rules are combined by nesting concat instances.
type concat struct {
	type1 Rule // The first rule that must match the beginning of the input
	type2 Rule // The second rule that must match after type1 consumes its portion
}

var _ Rule = &concat{}

// Concat creates a Rule that matches a sequence of rules in order.
// The resulting rule succeeds when the input can be partitioned into
// consecutive substrings, each matching the corresponding rule in sequence.
//
// Multiple rules are combined right-to-left internally: Concat(a, b, c)
// creates a structure equivalent to Concat(a, Concat(b, c)).
//
// Special cases:
//   - Concat() with no arguments returns a rule matching only empty strings
//   - Concat(rule) with one argument returns that rule unchanged
//
// Examples:
//
//	Concat(Range('0','9'), Range('a','z')) matches "5e" but not "AA" or "e5"
//	Concat(Terminal("ab"), Terminal("cd")) matches "abcd"
//
// Note: WithSuggestionFunc is not supported for Concat rules and will panic.
// To add suggestions, apply them to the individual component rules instead.
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

// validate matches the input against both constituent rules in sequence.
// First, all possible matches from type1 are collected. Then, for each match,
// type2 is applied to the remaining input. Results are yielded for each
// successful combination, with sanitized output being the concatenation
// of both rules' sanitized portions.
//
// A nil concat (from Concat() with no arguments) matches empty input only.
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

// WithSuggestionFunc panics when called on a Concat rule.
// Concat rules do not directly support suggestion functions.
// Apply suggestions to the individual component rules instead.
func (c *concat) WithSuggestionFunc(fn SuggestionFunc) Rule {
	panic("not implemented")
}

// alternative is a rule that succeeds if any one of its constituent rules matches.
// It attempts each rule in order and yields all successful matches,
// enabling ambiguous grammars where multiple interpretations are valid.
type alternative struct {
	types     []Rule         // The list of alternative rules, attempted in order
	suggestFn SuggestionFunc // Optional function to generate suggestions when no alternatives match
}

var _ Rule = &alternative{}

// Alternative creates a Rule that matches if any of the provided rules matches.
// All matching alternatives are explored, and results from each successful
// match are yielded, supporting ambiguous grammar interpretations.
//
// The rules are attempted in the order provided, though all matches are
// collected regardless of order.
//
// Examples:
//
//	Alternative(Range('a','z'), Range('A','Z')) matches any ASCII letter
//	Alternative(Terminal("cat"), Terminal("car")) matches either word
//
// The rule can be configured with WithSuggestionFunc to provide fallback
// suggestions when none of the alternatives match.
func Alternative(types ...Rule) Rule {
	return &alternative{
		types: types,
	}
}

// validate attempts to match the input against each alternative rule.
// Results from all successful matches are yielded. If no alternatives match
// and a suggestion function is configured, it is invoked to generate
// fallback results.
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

// WithSuggestionFunc attaches a suggestion function to this Alternative rule.
// The function is called only when none of the alternative rules match,
// providing fallback suggestions for sanitization.
// Returns the modified rule to enable method chaining.
func (a *alternative) WithSuggestionFunc(fn SuggestionFunc) Rule {
	a.suggestFn = fn
	return a
}

// namedTypes is the global registry mapping rule names to their implementations.
// This enables Named and RefNamed to create forward references and recursive rules.
// Access is protected by namedTypesLock for thread-safe concurrent operations.
var (
	namedTypes     = map[string]Rule{}
	namedTypesLock = sync.RWMutex{}
)

// named is a rule that references another rule by name.
// It enables lazy evaluation, forward references, and recursive grammar definitions
// by deferring the lookup of the actual rule until validation time.
type named struct {
	name string // The name of the referenced rule in the namedTypes registry
}

var _ Rule = &named{}

// Named registers a rule with the given name in the global registry and returns it.
// The named rule can later be referenced using RefNamed, enabling:
//   - Forward references: Reference a rule before it is defined
//   - Recursive rules: A rule that references itself directly or indirectly
//   - Rule reuse: Define a rule once and reference it multiple times
//
// Parameters:
//   - name: A unique identifier for this rule in the global registry
//   - rule: The validation rule to register
//
// Returns the provided rule unchanged, allowing inline usage.
//
// Example of a recursive rule for nested parentheses:
//
//	Named("parens", Alternative(
//	    Terminal("()"),
//	    Concat(Terminal("("), RefNamed("parens"), Terminal(")")),
//	))
func Named(name string, rule Rule) Rule {
	namedTypesLock.Lock()
	defer namedTypesLock.Unlock()
	namedTypes[name] = rule
	return rule
}

// randomNamePrefix is the prefix used when generating automatic unique rule names.
// Names starting with this prefix are reserved for internal use by GetRandomName.
const randomNamePrefix = "___random_name_"

// GetRandomName generates a unique name that is not currently registered in the global registry.
// This is used internally by Seq to create anonymous recursive rules, and can be used
// externally when a unique rule name is needed without manual coordination.
//
// The function generates names with the format "___random_name_<number>" where the number
// is derived from a cryptographically random starting point to minimize collisions.
//
// Panics if unable to find an unused name after 50,000 attempts, which would indicate
// an extremely congested registry.
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

// RefNamed creates a lazy reference to a rule registered with Named.
// The actual rule lookup is deferred until validation time, enabling:
//   - Forward references: Reference a rule that will be defined later
//   - Recursive rules: Reference the containing rule for self-recursion
//   - Mutual recursion: Two or more rules that reference each other
//
// The referenced rule must be registered via Named before or during validation.
// If the named rule does not exist at validation time, the reference yields no results.
//
// Note: WithSuggestionFunc is not supported for RefNamed rules and will panic.
// Apply suggestions to the actual named rule instead.
func RefNamed(name string) Rule {
	return &named{
		name: name,
	}
}

// validate looks up the named rule in the global registry and delegates validation to it.
// Implements cycle detection by checking if this rule with the same input has already
// been seen in the current call stack. If a cycle is detected, returns immediately
// with no results to prevent infinite recursion.
func (n *named) validate(ctx *parseContext, in string) <-chan result {
	np := namedPathNode{
		ruleName: n.name,
		input:    in,
	}

	if ctx.hasPathNode(np) {
		return closedCheckedChan
	}
	ctx.appendPathNode(np)
	namedTypesLock.RLock()
	defer namedTypesLock.RUnlock()
	if t, ok := namedTypes[n.name]; ok {
		return t.validate(ctx.clone(), in)
	}
	return closedCheckedChan
}

// WithSuggestionFunc panics when called on a RefNamed rule.
// Named references do not directly support suggestion functions.
// Apply suggestions to the actual named rule definition instead.
func (n *named) WithSuggestionFunc(fn SuggestionFunc) Rule {
	panic("not implemented")
}

// SeqInf is a sentinel value indicating infinite maximum repetitions in Seq rules.
// Use this as the max parameter when there should be no upper limit on repetitions.
const SeqInf = -1

// Seq creates a Rule that matches repeated consecutive occurrences of another rule.
// The number of repetitions must fall within the specified [min, max] range (inclusive).
//
// Parameters:
//   - min: Minimum required repetitions (negative values treated as 0)
//   - max: Maximum allowed repetitions (use SeqInf for unlimited, values < min are set to min)
//   - rule: The rule to be repeated
//
// Behavior for different parameter combinations:
//   - Seq(0, 0, rule): Matches only empty strings
//   - Seq(1, 1, rule): Equivalent to rule itself
//   - Seq(n, n, rule): Matches exactly n consecutive occurrences
//   - Seq(0, n, rule): Matches 0 to n occurrences (optional up to n times)
//   - Seq(n, SeqInf, rule): Matches n or more occurrences (no upper limit)
//
// Examples:
//
//	Seq(5, 5, Terminal("a"))     // Matches exactly "aaaaa"
//	Seq(1, 3, Terminal("b"))     // Matches "b", "bb", or "bbb"
//	Seq(0, 1, rule)              // Equivalent to Opt(rule)
//	Seq(1, SeqInf, Terminal("c")) // Matches "c", "cc", "ccc", etc.
//	Seq(0, SeqInf, rule)         // Matches any number of occurrences (Kleene star)
//
// Implementation note: Infinite repetitions (SeqInf) are implemented using
// recursive named rules, which automatically handles cycle detection.
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

// Opt creates a Rule that matches zero or one occurrence of the given rule.
// This is equivalent to Seq(0, 1, rule) and represents optional content in a grammar.
//
// Examples:
//
//	Opt(Terminal("-"))           // Matches "" or "-" (optional minus sign)
//	Concat(Opt(Terminal("+")), Range('0','9')) // Optional plus before digit
//
// The rule succeeds with an empty match if the input doesn't match,
// or with the full match if it does match.
func Opt(rule Rule) Rule {
	return Seq(0, 1, rule)
}
