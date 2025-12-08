package parsan

import "strings"

// Digit returns a rule that matches a single ASCII digit character (0-9).
// The optional suggestFn parameter provides custom suggestions when
// validation fails on invalid input.
func Digit(suggestFn SuggestionFunc) Rule {
	return Range('0', '9').WithSuggestionFunc(suggestFn)
}

// Letter returns a rule that matches a single ASCII letter character,
// either lowercase (a-z) or uppercase (A-Z).
// The optional suggestFn parameter provides custom suggestions when
// validation fails on invalid input.
func Letter(suggestFn SuggestionFunc) Rule {
	return Alternative(
		Range('a', 'z'),
		Range('A', 'Z'),
	).WithSuggestionFunc(suggestFn)
}

// suggestLowerLetter is a suggestion function that converts the first
// character of the input to lowercase if it is an ASCII letter.
// Returns nil if the input is empty or the first character is not a letter.
func suggestLowerLetter(in string) []*result {
	if len(in) == 0 {
		return nil
	}
	first := in[0]
	var lowerFirst string
	switch {
	case first >= 'a' && first <= 'z':
		lowerFirst = string(first)
	case first >= 'A' && first <= 'Z':
		lowerFirst = strings.ToLower(string(first))
	default:
		return nil
	}

	return []*result{
		{
			sanitized: lowerFirst,
			toParse:   in[1:],
		},
	}
}

// LowerLetter returns a rule that matches a single lowercase ASCII letter (a-z).
// When validation fails, it first attempts to suggest converting uppercase
// letters to lowercase via suggestLowerLetter, then falls back to the
// optional suggestFn for other invalid characters.
func LowerLetter(suggestFn SuggestionFunc) Rule {
	return Range('a', 'z').WithSuggestionFunc(UnlessSuggestionFunc(
		suggestLowerLetter,
		suggestFn,
	))
}

// LetDig returns a rule that matches a single alphanumeric ASCII character,
// which includes digits (0-9), lowercase letters (a-z), and uppercase
// letters (A-Z).
// The optional suggestFn parameter provides custom suggestions when
// validation fails on invalid input.
func LetDig(suggestFn SuggestionFunc) Rule {
	return Alternative(
		Letter(nil),
		Digit(nil),
	).WithSuggestionFunc(suggestFn)
}

// LowerLetDig returns a rule that matches a single lowercase alphanumeric
// ASCII character, which includes digits (0-9) and lowercase letters (a-z).
// The optional suggestFn parameter provides custom suggestions when
// validation fails on invalid input.
func LowerLetDig(suggestFn SuggestionFunc) Rule {
	return Alternative(
		LowerLetter(nil),
		Digit(nil),
	).WithSuggestionFunc(suggestFn)
}

// LetDigHyp returns a rule that matches a single ASCII character that is
// either alphanumeric (0-9, a-z, A-Z) or a hyphen ('-').
// This is commonly used for parsing domain name label characters.
// The optional suggestFn parameter provides custom suggestions when
// validation fails on invalid input.
func LetDigHyp(suggestFn SuggestionFunc) Rule {
	return Alternative(
		LetDig(nil),
		Terminal("-")).
		WithSuggestionFunc(suggestFn)
}

// LowerLetDigHyp returns a rule that matches a single ASCII character that
// is either lowercase alphanumeric (0-9, a-z) or a hyphen ('-').
// This is useful for parsing lowercase domain name label characters.
// The optional suggestFn parameter provides custom suggestions when
// validation fails on invalid input.
func LowerLetDigHyp(suggestFn SuggestionFunc) Rule {
	return Alternative(
		LowerLetDig(nil),
		Terminal("-")).
		WithSuggestionFunc(suggestFn)
}

// LDHStr returns a rule that matches a string of one or more LDH
// (Letter-Digit-Hyphen) characters. This corresponds to the "ldh-str"
// production in RFC 1035 for DNS domain name labels.
// The rule is defined recursively using a named reference.
// The optional suggestFn parameter provides custom suggestions when
// validation fails on invalid input.
func LDHStr(suggestFn SuggestionFunc) Rule {
	name := GetRandomName()
	return Named(name,
		Alternative(
			LetDigHyp(suggestFn),
			Concat(
				LetDigHyp(suggestFn),
				RefNamed(name),
			),
		),
	)
}

// LowerLDHStr returns a rule that matches a string of one or more lowercase
// LDH (Letter-Digit-Hyphen) characters, where letters must be lowercase (a-z).
// The rule is defined recursively using a named reference.
// The optional suggestFn parameter provides custom suggestions when
// validation fails on invalid input.
func LowerLDHStr(suggestFn SuggestionFunc) Rule {
	name := GetRandomName()
	return Named(name,
		Alternative(
			LowerLetDigHyp(suggestFn),
			Concat(
				LowerLetDigHyp(suggestFn),
				RefNamed(name),
			),
		),
	)
}
