package parsan

// SuggestionFunc defines a function type that generates parsing suggestions.
// It takes an input string and returns a slice of result pointers representing
// alternative parsing interpretations. This type is typically passed to
// WithSuggestionFunc methods on Rule types to customize suggestion behavior.
type SuggestionFunc func(string) []*result

// UnlessSuggestionFunc creates a conditional SuggestionFunc that first attempts
// to generate suggestions using unlessFn. If unlessFn returns no results,
// it falls back to thenFn. This allows for prioritized suggestion strategies
// where one approach is preferred but another serves as a fallback.
func UnlessSuggestionFunc(unlessFn, thenFn SuggestionFunc) SuggestionFunc {
	return func(in string) []*result {
		results := unlessFn(in)
		if len(results) == 0 && thenFn != nil {
			return thenFn(in)
		}
		return results
	}
}

// MergeSuggestionFuncs combines multiple SuggestionFunc functions into a single
// SuggestionFunc. The returned function invokes each provided function in order
// and concatenates all their results into a single slice. This is useful for
// aggregating suggestions from multiple independent sources.
func MergeSuggestionFuncs(fns ...SuggestionFunc) SuggestionFunc {
	return func(in string) []*result {
		checked := make([]*result, 0)
		for _, fn := range fns {
			checked = append(checked, fn(in)...)
		}
		return checked
	}
}

// SuggestConstRune creates a SuggestionFunc that suggests replacing the first
// rune of the input with the specified rune r. This is a convenience wrapper
// around ReplaceFirstRuneWithStrings for single-rune replacements.
func SuggestConstRune(r rune) SuggestionFunc {
	return ReplaceFirstRuneWithStrings(string(r))
}

// PrependOrReplaceFirstRuneWithStrings creates a SuggestionFunc that generates
// two types of suggestions for each provided string:
//  1. Prepending the string to the entire input (insertion before input)
//  2. Replacing the first rune of the input with the string (substitution)
//
// For an input "abc" and string "X", this produces suggestions for parsing
// "X" with remainder "abc", and "X" with remainder "bc". Returns up to
// 2*len(ss) results, with the replacement variant omitted for empty input.
func PrependOrReplaceFirstRuneWithStrings(ss ...string) SuggestionFunc {
	return func(in string) []*result {
		checkeds := make([]*result, 0, 2*len(ss))
		for _, s := range ss {
			checkeds = append(checkeds, &result{
				sanitized: s,
				toParse:   in,
			})
			if len(in) > 0 {
				checkeds = append(checkeds, &result{
					sanitized: s,
					toParse:   in[1:],
				})
			}
		}
		return checkeds
	}
}

// ReplaceFirstRuneWithStrings creates a SuggestionFunc that generates suggestions
// by substituting the first byte of the input with each of the provided strings.
// Each result contains the replacement string as the sanitized portion and the
// remaining input (after the first byte) as the portion still to be parsed.
// Returns nil if the input is empty, as there is no character to replace.
func ReplaceFirstRuneWithStrings(ss ...string) SuggestionFunc {
	return func(in string) []*result {
		if len(in) == 0 {
			return nil
		}
		remaining := ""
		if len(in) > 1 {
			remaining = in[1:]
		}
		checkeds := make([]*result, len(ss))
		for i, s := range ss {
			checkeds[i] = &result{
				sanitized: s,
				toParse:   remaining,
			}
		}
		return checkeds
	}
}
