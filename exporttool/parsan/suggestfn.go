package parsan

// SuggestionFunc represents a function that generates parsing suggestions
// by returning alternative representations for a given input string.
// Used as a parameter for the WithSuggestionFunc method of Rule types.
type SuggestionFunc func(string) []*result

// MergeSuggestionFuncs combines multiple SuggestionFunc functions into a single
// SuggestionFunc that returns the concatenated results of all provided functions.
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
// rune of the input string with the specified rune 'r'.
func SuggestConstRune(r rune) SuggestionFunc {
	return ReplaceFirstRuneWithStrings(string(r))
}

// PrependOrReplaceFirstRuneWithStrings creates a SuggestionFunc that generates suggestions
// by prepending each specified string to the input, and optionally replacing
// the first rune with each string followed by the remainder of the input.
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
// by replacing the first rune of the input string with each of the specified strings.
// Returns nil if the input string is empty.
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
