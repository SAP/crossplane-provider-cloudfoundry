package parsan

import "strings"

// Digit returns a rule that matches a single ASCII digit character (0-9).
// If suggestFn is provided, it will be called to generate suggestions when
// the rule fails to match the input.
func Digit(suggestFn SuggestionFunc) Rule {
	return Range('0', '9').WithSuggestionFunc(suggestFn)
}

// Letter returns a rule that matches a single ASCII letter character.
// Both lowercase (a-z) and uppercase (A-Z) letters are accepted.
// If suggestFn is provided, it will be called to generate suggestions when
// the rule fails to match the input.
func Letter(suggestFn SuggestionFunc) Rule {
	return Alternative(
		Range('a', 'z'),
		Range('A', 'Z'),
	).WithSuggestionFunc(suggestFn)
}

// suggestLowerLetter generates a suggestion by converting the first character
// of the input to lowercase. This enables automatic case correction during
// parsing.
//
// If the first character is already lowercase (a-z), it returns that character.
// If the first character is uppercase (A-Z), it returns its lowercase equivalent.
// If the input is empty or the first character is not a letter, it returns nil.
func suggestLowerLetter(in string) []*parseResult {
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

	return []*parseResult{
		{
			consumedText: lowerFirst,
			rest:         in[1:],
		},
	}
}

// LowerLetter returns a rule that matches a single lowercase ASCII letter (a-z).
//
// When matching fails, the rule uses a two-stage suggestion strategy:
//  1. First, suggestLowerLetter attempts to convert an uppercase letter to lowercase.
//  2. If that fails (e.g., the character is not a letter), suggestFn is invoked.
//
// If suggestFn is nil, only the uppercase-to-lowercase conversion is suggested.
func LowerLetter(suggestFn SuggestionFunc) Rule {
	return Range('a', 'z').WithSuggestionFunc(UnlessSuggestionFunc(
		suggestLowerLetter,
		suggestFn,
	))
}

// LetDig returns a rule that matches a single alphanumeric ASCII character.
// Valid characters include digits (0-9), lowercase letters (a-z), and
// uppercase letters (A-Z).
// If suggestFn is provided, it will be called to generate suggestions when
// the rule fails to match the input.
func LetDig(suggestFn SuggestionFunc) Rule {
	return Alternative(
		Letter(nil),
		Digit(nil),
	).WithSuggestionFunc(suggestFn)
}

// LowerLetDig returns a rule that matches a single lowercase alphanumeric
// ASCII character. Valid characters include digits (0-9) and lowercase
// letters (a-z). Uppercase letters are not matched directly but may be
// suggested as their lowercase equivalents via the LowerLetter rule.
// If suggestFn is provided, it will be called to generate suggestions when
// the rule fails to match the input.
func LowerLetDig(suggestFn SuggestionFunc) Rule {
	return Alternative(
		LowerLetter(nil),
		Digit(nil),
	).WithSuggestionFunc(suggestFn)
}

// LetDigHyp returns a rule that matches a single LDH (Letter-Digit-Hyphen)
// character. Valid characters include alphanumeric characters (0-9, a-z, A-Z)
// and the hyphen character ('-').
//
// This rule implements the character set used in DNS domain name labels as
// defined in RFC 1035.
//
// If suggestFn is provided, it will be called to generate suggestions when
// the rule fails to match the input.
func LetDigHyp(suggestFn SuggestionFunc) Rule {
	return Alternative(
		LetDig(nil),
		Terminal("-")).
		WithSuggestionFunc(suggestFn)
}

// LowerLetDigHyp returns a rule that matches a single lowercase LDH
// (Letter-Digit-Hyphen) character. Valid characters include digits (0-9),
// lowercase letters (a-z), and the hyphen character ('-').
//
// This is useful for parsing domain name labels in a case-normalized form.
//
// If suggestFn is provided, it will be called to generate suggestions when
// the rule fails to match the input.
func LowerLetDigHyp(suggestFn SuggestionFunc) Rule {
	return Alternative(
		LowerLetDig(nil),
		Terminal("-")).
		WithSuggestionFunc(suggestFn)
}

// LDHStr returns a rule that matches one or more consecutive LDH
// (Letter-Digit-Hyphen) characters. This corresponds to the "ldh-str"
// production in RFC 1035 for DNS domain name labels.
//
// The rule is implemented recursively using a unique named reference to
// handle strings of arbitrary length.
//
// If suggestFn is provided, it will be called to generate suggestions when
// the rule fails to match the input.
func LDHStr(suggestFn SuggestionFunc) Rule {
	name := GenerateUniqueName()
	return Named(name,
		Alternative(
			LetDigHyp(suggestFn),
			Concat(
				LetDigHyp(suggestFn),
				Ref(name),
			),
		),
	)
}

// LowerLDHStr returns a rule that matches one or more consecutive lowercase
// LDH (Letter-Digit-Hyphen) characters. Valid characters include digits (0-9),
// lowercase letters (a-z), and hyphens ('-').
//
// The rule is implemented recursively using a unique named reference to
// handle strings of arbitrary length.
//
// If suggestFn is provided, it will be called to generate suggestions when
// the rule fails to match the input.
func LowerLDHStr(suggestFn SuggestionFunc) Rule {
	name := GenerateUniqueName()
	return Named(name,
		Alternative(
			LowerLetDigHyp(suggestFn),
			Concat(
				LowerLetDigHyp(suggestFn),
				Ref(name),
			),
		),
	)
}
