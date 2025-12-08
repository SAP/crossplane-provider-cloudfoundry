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
	emptyResultChan = make(chan parseResult)
	close(emptyResultChan)
}

// emptyResultChan is a pre-closed channel that signals immediate termination
// of parsing. It is returned from match() when no valid parse results can exist,
// such as when cycle detection identifies infinite recursion or when a
// referenced rule does not exist in the global registry.
var emptyResultChan chan parseResult

// callStackEntry represents a single frame in the parsing call stack.
// It captures the rule name and the exact input string being parsed at that point.
// Two entries are considered equal if both the rule name and input match exactly,
// which indicates a recursive cycle that would lead to infinite recursion.
type callStackEntry struct {
	ruleName string
	input    string
}

// validationContext maintains state during recursive descent parsing.
// It tracks the sequence of named rule invocations along with their inputs
// to detect and terminate infinite recursion cycles. Each parsing branch
// maintains its own independent context to correctly identify cycles
// within that specific execution path.
type validationContext struct {
	namedPath []callStackEntry
}

// newParseContext creates a fresh validation context for initiating a new
// top-level parse operation. The returned context has an empty call stack.
func newParseContext() *validationContext {
	return &validationContext{
		namedPath: []callStackEntry{},
	}
}

// hasVisited checks whether the given rule and input combination already
// appears in the current call stack. Returns true if a matching entry exists,
// indicating that continuing would cause infinite recursion since the parser
// would process the same rule with the same input indefinitely.
func (pc *validationContext) hasVisited(node callStackEntry) bool {
	return slices.ContainsFunc(pc.namedPath, func(np callStackEntry) bool {
		return np.input == node.input && np.ruleName == node.ruleName
	})
}

// pushToStack appends a new entry to the call stack. This should be called
// when entering a named rule to record the invocation for subsequent cycle
// detection checks.
func (pc *validationContext) pushToStack(node callStackEntry) {
	pc.namedPath = append(pc.namedPath, node)
}

// clone creates a deep copy of the validation context. This is necessary
// when exploring multiple parsing branches in parallel, as each branch
// requires its own independent call stack to correctly track cycles
// within that specific execution path without interference from sibling branches.
func (pc *validationContext) clone() *validationContext {
	return &validationContext{
		namedPath: slices.Clone(pc.namedPath),
	}
}

// parseResult encapsulates a single possible outcome from parsing.
// Since grammars may be ambiguous, a single input can yield multiple
// valid parse results, each representing a different interpretation
// of the input string.
type parseResult struct {
	consumedText string // The portion of input that was successfully matched and potentially transformed
	rest         string // The remaining unparsed suffix of the input
}

// ParseAndSanitize validates the input string against the given rule and
// returns all valid interpretations as sanitized strings.
//
// The function performs the following steps:
//  1. Truncates the input if the rule specifies a maximum length constraint
//  2. Executes the rule's match function to collect all possible parse results
//  3. Filters results to include only those that fully consume the input (empty rest)
//  4. Deduplicates identical results
//  5. Sorts results by length in descending order, with alphabetical ordering as tiebreaker
//
// Returns an empty slice if no valid complete parse exists for the input.
func ParseAndSanitize(input string, rule Rule) []string {
	if mlen := rule.maxLength(); mlen != Unlimited && mlen < len(input) {
		input = input[:mlen]
	}
	checked := rule.match(newParseContext(), input)
	suggestions := map[string]struct{}{}
	for ch := range checked {
		if len(ch.rest) == 0 {
			suggestions[ch.consumedText] = struct{}{}
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

// Rule defines the interface for all grammar rule types in the parser.
// Rules are composable building blocks that can be combined to construct
// complex grammars for input validation and sanitization.
//
// The parser uses a recursive descent approach with backtracking, where
// each rule can produce multiple possible parse results through a channel.
// This design supports ambiguous grammars where multiple valid interpretations
// may exist for a single input.
//
// Built-in rule implementations include:
//   - Terminal: matches exact string literals
//   - Range: matches single characters within Unicode code point bounds
//   - Concat: matches a sequence of rules in order
//   - Alternative: matches any one of several possible rules
//   - Named/Ref: enables recursive grammars and forward references
//   - Seq: matches repeated occurrences of a rule (with min/max bounds)
//   - Opt: matches zero or one occurrence of a rule
type Rule interface {
	// match attempts to parse the input string according to this rule.
	// It returns a channel that yields all possible parse results.
	// The validation context is used to track recursive rule invocations
	// and detect infinite cycles. The channel is closed when all
	// possible results have been yielded.
	match(*validationContext, string) <-chan parseResult

	// WithSuggestionFunc attaches a fallback suggestion generator that is
	// invoked when the rule fails to match the input. Not all rule types
	// support suggestions; calling this on an unsupported type will panic.
	WithSuggestionFunc(SuggestionFunc) Rule

	// WithMaxLength sets the maximum allowed length for matched content.
	// Results exceeding this length will be truncated or filtered.
	// Returns the modified rule for method chaining.
	WithMaxLength(maxLength int) Rule

	// maxLength returns the maximum length constraint for this rule,
	// or Unlimited if no constraint is set.
	maxLength() int
}

// terminal is a rule that matches an exact string literal at the beginning
// of the input. It is the most basic building block for constructing grammars.
type terminal struct {
	text   string
	maxLen int
}

var _ Rule = &terminal{}

// Terminal creates a rule that matches the exact string s at the start of
// the input. On successful match, it consumes exactly len(s) characters
// and leaves the remainder for subsequent rules to process.
func Terminal(s string) Rule {
	return &terminal{
		text:   s,
		maxLen: Unlimited,
	}
}

// match checks if the input begins with this terminal's text.
// Yields exactly one result if the prefix matches, or no results otherwise.
// The consumed text is the terminal string itself, and the rest is the
// input with that prefix removed.
func (t *terminal) match(vctx *validationContext, input string) <-chan parseResult {
	out := make(chan parseResult)
	go func() {
		defer close(out)
		if remaining, matches := strings.CutPrefix(input, t.text); matches {
			out <- parseResult{
				consumedText: t.text,
				rest:         remaining,
			}
		}
	}()
	return out
}

// WithSuggestionFunc is not supported for Terminal rules because the literal
// text is the only valid match. Calling this method will panic.
func (t *terminal) WithSuggestionFunc(suggester SuggestionFunc) Rule {
	panic("not implemented")
}

// WithMaxLength sets the maximum length constraint for this terminal.
// Panics if the specified length does not equal the terminal's text length,
// as a terminal can only match its exact string.
func (t *terminal) WithMaxLength(maxLength int) Rule {
	if len(t.text) != maxLength {
		panic(fmt.Sprintf("maxLength (%d) of terminal rule (%s) mismatch",
			maxLength,
			t.text))
	}
	t.maxLen = maxLength
	return t
}

// maxLength returns the maximum length constraint, or Unlimited if none is set.
func (t *terminal) maxLength() int {
	return t.maxLen
}

// rangeType is a rule that matches a single character if its Unicode code point
// falls within the inclusive range [minChar, maxChar]. This enables matching
// character classes like digits (0-9) or letters (a-z).
type rangeType struct {
	minChar   rune
	maxChar   rune
	suggestFn SuggestionFunc
	maxLen    int
}

var _ Rule = &rangeType{}

// Range creates a rule that matches a single character whose Unicode code point
// is between start and end, inclusive. If end is less than start, the range
// is normalized to match only the start character.
//
// Use WithSuggestionFunc to provide alternative suggestions when the input
// character falls outside the valid range.
func Range(start, end rune) Rule {
	if end < start {
		end = start
	}
	return &rangeType{
		minChar: start,
		maxChar: end,
		maxLen:  Unlimited,
	}
}

// match checks if the first character of input falls within the range.
// If it does, yields a result consuming that single character.
// If no match occurs and a suggestion function is configured, invokes
// the suggestion function to generate alternative parse results.
func (r *rangeType) match(vctx *validationContext, input string) <-chan parseResult {
	out := make(chan parseResult)
	suggested := false
	go func() {
		defer close(out)
		if len(input) > 0 {
			first := rune(input[0])
			if first >= r.minChar && first <= r.maxChar {
				suggested = true
				out <- parseResult{
					consumedText: string(first),
					rest:         input[1:],
				}
			}
		}
		if !suggested && r.suggestFn != nil {
			for _, checked := range r.suggestFn(input) {
				if checked != nil {
					out <- *checked
				}
			}
		}
	}()
	return out
}

// WithSuggestionFunc attaches a fallback generator that is invoked when
// the input character does not fall within the valid range. The suggestion
// function can provide alternative valid characters or transformations.
// Returns the modified rule for method chaining.
func (r *rangeType) WithSuggestionFunc(suggester SuggestionFunc) Rule {
	r.suggestFn = suggester
	return r
}

// WithMaxLength sets the maximum length constraint. For Range rules,
// this must be exactly 1 since the rule matches only a single character.
// Panics if any other value is provided.
func (r *rangeType) WithMaxLength(maxLength int) Rule {
	if maxLength != 1 {
		panic(fmt.Sprintf("invalid maxLength (%d) for range rule",
			maxLength))
	}
	r.maxLen = maxLength
	return r
}

// maxLength returns the maximum length constraint, or Unlimited if none is set.
func (r *rangeType) maxLength() int {
	return r.maxLen
}

// concat is a rule that matches two sub-rules in sequence. The first rule
// must match some prefix of the input, then the second rule must match
// some prefix of the remainder. Multiple rules are combined by nesting
// concat instances.
type concat struct {
	first  Rule
	second Rule
	maxLen int
}

var _ Rule = &concat{}

// Concat creates a rule that matches multiple rules in sequential order.
// The input is partitioned into consecutive segments, where each segment
// matches the corresponding rule in order.
//
// The function handles special cases:
//   - Zero arguments: returns a rule that matches only empty strings (nil concat)
//   - One argument: returns that rule unchanged (no wrapping needed)
//   - Multiple arguments: combines rules right-to-left into nested concat pairs
//
// WithSuggestionFunc is not supported for Concat; apply suggestions to
// the individual component rules instead.
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
			first:  rules[len(rules)-2],
			second: rules[len(rules)-1],
			maxLen: Unlimited,
		}
		rules = append(rules[:len(rules)-2], merged)
	}
	return merged
}

// match first collects all possible results from the first rule, then
// for each of those results, applies the second rule to the remaining input.
// Yields all combinations where both rules successfully match in sequence.
//
// A nil concat (created by Concat() with no arguments) matches only the
// empty string, yielding a single result with empty consumed text and
// the entire input as the remainder.
func (c *concat) match(vctx *validationContext, input string) <-chan parseResult {
	out := make(chan parseResult)
	go func() {
		defer close(out)
		if c != nil {
			firstRuleResults := []parseResult{}
			for checked := range c.first.match(vctx.clone(), input) {
				if c.maxLen == Unlimited || len(checked.consumedText) <= c.maxLen {
					firstRuleResults = append(firstRuleResults, checked)
				} else {
					firstRuleResults = append(firstRuleResults, parseResult{
						consumedText: checked.consumedText[:c.maxLen],
						rest:         checked.rest,
					})
				}
			}
			for _, firstRuleResult := range firstRuleResults {
				for secondRuleResult := range c.second.match(vctx.clone(), firstRuleResult.rest) {
					consumedText := firstRuleResult.consumedText + secondRuleResult.consumedText
					if c.maxLen == Unlimited || len(firstRuleResult.consumedText)+len(secondRuleResult.consumedText) <= c.maxLen {
						out <- parseResult{
							consumedText: consumedText,
							rest:         secondRuleResult.rest,
						}
					} else {
						out <- parseResult{
							consumedText: consumedText[:c.maxLen],
							rest:         secondRuleResult.rest,
						}
					}
				}
			}
		} else {
			out <- parseResult{
				consumedText: "",
				rest:         input,
			}
		}
	}()
	return out
}

// WithSuggestionFunc is not supported for Concat rules because suggestions
// should be applied to individual component rules. Calling this method will panic.
func (c *concat) WithSuggestionFunc(suggester SuggestionFunc) Rule {
	panic("not implemented")
}

// WithMaxLength sets the maximum combined length for the concatenated match.
// Results exceeding this length will be truncated. An empty concat (nil)
// can only have a maxLength of 0. Panics on negative values or invalid
// constraints for empty concat.
func (c *concat) WithMaxLength(maxLength int) Rule {
	if c == nil {
		if maxLength != 0 {
			panic("empty concat rule cannot have maxLength other than 0")
		}
		return c
	}
	if maxLength < 0 {
		panic(fmt.Sprintf("invalid maxLength (%d) for concat rule",
			maxLength))
	}
	c.maxLen = maxLength
	return c
}

// maxLength returns the maximum length constraint for the concatenated result,
// or Unlimited if none is set. A nil concat returns Unlimited.
func (c *concat) maxLength() int {
	if c == nil {
		return Unlimited
	}
	return c.maxLen
}

// alternative is a rule that succeeds if any one of its constituent rules
// matches the input. It collects results from all matching alternatives,
// supporting ambiguous grammars where multiple interpretations are valid.
type alternative struct {
	options   []Rule
	suggestFn SuggestionFunc
	maxLen    int
}

var _ Rule = &alternative{}

// Alternative creates a rule that matches if any of the provided rules match.
// All matching alternatives are explored and their results are yielded,
// enabling the parser to handle ambiguous grammars with multiple valid
// interpretations of the same input.
//
// Use WithSuggestionFunc to provide fallback suggestions when none of
// the alternatives match.
func Alternative(types ...Rule) Rule {
	return &alternative{
		options: types,
		maxLen:  Unlimited,
	}
}

// match attempts each alternative rule in order and yields results from
// all successful matches. If no alternatives match and a suggestion function
// is configured, invokes the suggestion function to generate fallback results.
// Results exceeding the maximum length constraint are filtered out.
//
//gocyclo:ignore
func (a *alternative) match(vctx *validationContext, input string) <-chan parseResult {
	out := make(chan parseResult)
	go func() {
		defer close(out)
		validated := false
		for _, t := range a.options {
			for checked := range t.match(vctx.clone(), input) {
				if a.maxLen == Unlimited || len(checked.consumedText) <= a.maxLen {
					out <- checked
					validated = true
				}
			}
		}
		if a.suggestFn != nil && !validated {
			for _, checked := range a.suggestFn(input) {
				if a.maxLen == Unlimited || len(checked.consumedText) <= a.maxLen {
					if checked != nil {
						out <- *checked
					}
				}
			}
		}
	}()
	return out
}

// WithSuggestionFunc attaches a fallback generator that is invoked when
// none of the alternatives match the input. The suggestion function can
// provide alternative valid interpretations or corrections.
// Returns the modified rule for method chaining.
func (a *alternative) WithSuggestionFunc(suggester SuggestionFunc) Rule {
	a.suggestFn = suggester
	return a
}

// WithMaxLength sets the maximum length constraint for matched content.
// Alternative results exceeding this length will be filtered out.
// Panics on negative values.
func (a *alternative) WithMaxLength(maxLength int) Rule {
	if maxLength < 0 {
		panic(fmt.Sprintf("invalid maxLength (%d) for alternative rule",
			maxLength))
	}
	a.maxLen = maxLength
	return a
}

// maxLength returns the maximum length constraint, or Unlimited if none is set.
func (a *alternative) maxLength() int {
	return a.maxLen
}

// namedRules is the global registry that maps rule names to their implementations.
// This enables forward references and recursive grammar definitions.
// Access is protected by registryMutex for thread safety.
var (
	namedRules    = map[string]Rule{}
	registryMutex = sync.RWMutex{}
)

// named is a lazy reference to a rule registered in the global registry.
// It defers the actual rule lookup until match time, enabling forward
// references (using a rule before it is defined) and recursive grammars
// (a rule that directly or indirectly references itself).
type named struct {
	name string
}

var _ Rule = &named{}

// Named registers a rule in the global registry under the specified name
// and returns the rule unchanged. This enables other rules to reference
// this rule by name using Ref, supporting:
//   - Forward references: use a rule before defining it in the source code
//   - Recursive grammars: a rule that references itself directly or indirectly
//   - Reusability: define a rule once and reference it from multiple places
func Named(name string, rule Rule) Rule {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	namedRules[name] = rule
	return rule
}

// autoGeneratedNamePrefix is the prefix used for automatically generated
// unique rule names. Names beginning with this prefix are reserved for
// internal use by GenerateUniqueName and should not be used directly.
const autoGeneratedNamePrefix = "___random_name_"

// GenerateUniqueName creates a unique name that does not exist in the
// global rule registry. This is used internally by Seq to create named
// rules for unbounded repetition patterns.
//
// The function generates names by combining the reserved prefix with
// a random offset plus an incrementing counter. It panics after 50,000
// failed attempts, which would indicate severe registry congestion
// (an unlikely scenario in normal usage).
func GenerateUniqueName() string {
	suffixBigNum, err := rand.Int(rand.Reader, big.NewInt(10000000000))
	if err != nil {
		panic(err)
	}
	suffixNum := suffixBigNum.Int64()
	for i := range 50000 {
		name := fmt.Sprintf("%s%d", autoGeneratedNamePrefix, i+int(suffixNum))
		if _, ok := namedRules[name]; !ok {
			return name
		}
	}
	panic("cannot get random name")
}

// Ref creates a lazy reference to a rule registered via Named.
// The actual rule lookup is deferred until match time, which allows:
//   - Forward references: reference a rule before it is registered
//   - Recursive definitions: a rule can reference itself through a Ref
//
// If the referenced rule is not registered when match is called,
// the Ref yields no results (returns emptyResultChan).
//
// WithSuggestionFunc is not supported for Ref; apply suggestions
// to the target rule instead.
func Ref(name string) Rule {
	return &named{
		name: name,
	}
}

// match looks up the referenced rule in the global registry and delegates
// parsing to it. Before delegating, it checks for recursive cycles by
// examining whether this rule with this exact input already appears in
// the call stack. If a cycle is detected, it returns emptyResultChan
// immediately to prevent infinite recursion.
//
// Returns emptyResultChan if the referenced rule is not registered.
func (n *named) match(vctx *validationContext, input string) <-chan parseResult {
	np := callStackEntry{
		ruleName: n.name,
		input:    input,
	}

	if vctx.hasVisited(np) {
		return emptyResultChan
	}
	vctx.pushToStack(np)
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	if t, ok := namedRules[n.name]; ok {
		return t.match(vctx.clone(), input)
	}
	return emptyResultChan
}

// WithSuggestionFunc is not supported for Ref rules because suggestions
// should be applied to the target rule being referenced. Calling this
// method will panic.
func (n *named) WithSuggestionFunc(suggester SuggestionFunc) Rule {
	panic("not implemented")
}

// WithMaxLength delegates the max length setting to the referenced rule.
// Looks up the rule in the registry and calls WithMaxLength on it.
// Returns nil if the referenced rule is not registered.
func (n *named) WithMaxLength(maxLength int) Rule {
	if rule, ok := namedRules[n.name]; ok {
		return rule.WithMaxLength(maxLength)
	}
	return nil
}

// maxLength returns the max length constraint of the referenced rule,
// or Unlimited if the rule is not registered.
func (n *named) maxLength() int {
	if rule, ok := namedRules[n.name]; ok {
		return rule.maxLength()
	}
	return Unlimited
}

// Unlimited is a sentinel value indicating no upper bound on repetitions
// or length constraints. Used with Seq for unbounded repetition and with
// maxLength to indicate no length limit.
const Unlimited = -1

// Seq creates a rule that matches between min and max consecutive occurrences
// of the given rule. This is the general-purpose repetition combinator.
//
// Parameters:
//   - min: minimum number of repetitions required (negative values are treated as 0)
//   - max: maximum number of repetitions allowed (use Unlimited for no upper bound;
//     values less than min are normalized to min)
//   - rule: the rule to be repeated
//
// Implementation details:
//   - For fixed counts (min == max): creates a simple concatenation of that many copies
//   - For unbounded max: creates a recursive grammar using Named/Ref
//   - For bounded ranges: creates an Alternative of all valid repetition counts
//
// Common patterns:
//   - Seq(0, 0, r): matches only empty string
//   - Seq(1, 1, r): equivalent to just r
//   - Seq(0, 1, r): equivalent to Opt(r), matches zero or one
//   - Seq(2, 5, r): matches 2, 3, 4, or 5 consecutive occurrences
//   - Seq(1, Unlimited, r): Kleene plus, matches one or more
//   - Seq(0, Unlimited, r): Kleene star, matches zero or more
func Seq(min, max int, rule Rule) Rule {
	if min < 0 {
		min = 0
	}
	if max < 0 && max != Unlimited {
		max = Unlimited
	}
	if max != Unlimited && max < min {
		max = min
	}
	if min == max {
		if min == 1 {
			return rule
		}
		return Concat(slices.Repeat[[]Rule]([]Rule{rule}, min)...)
	}
	if max == Unlimited {
		prefix := Seq(min, min, rule)
		ruleName := GenerateUniqueName()
		suffix := Named(ruleName,
			Alternative(
				Concat(),
				rule,
				Concat(rule,
					Ref(ruleName)),
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

// Opt creates a rule that matches zero or one occurrence of the given rule.
// This is a convenience function equivalent to Seq(0, 1, rule).
//
// The rule always succeeds: it yields an empty match (consuming nothing)
// and, if the input matches the rule, also yields the full match. This
// makes the wrapped rule optional in the grammar.
func Opt(rule Rule) Rule {
	return Seq(0, 1, rule)
}
