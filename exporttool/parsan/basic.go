package parsan

// Digit returns a rule that validates a single digit character (0-9).
// The suggestFn parameter allows providing custom suggestions for invalid input.
func Digit(suggestFn SuggestionFunc) Rule {
	return Range('0', '9').WithSuggestionFunc(suggestFn)
}

// Letter returns a rule that validates a single letter character (a-z or A-Z).
// The suggestFn parameter allows providing custom suggestions for invalid input.
func Letter(suggestFn SuggestionFunc) Rule {
	return Alternative(
		Range('a', 'z'),
		Range('A', 'Z'),
	).WithSuggestionFunc(suggestFn)
}

// LetDig returns a rule that validates a single alphanumeric character (0-9, a-z, or A-Z).
// The suggestFn parameter allows providing custom suggestions for invalid input.
func LetDig(suggestFn SuggestionFunc) Rule {
	return Alternative(
		Letter(nil),
		Digit(nil),
	).WithSuggestionFunc(suggestFn)
}

// LetDigHyp returns a rule that validates a single character that is alphanumeric or hyphen (0-9, a-z, A-Z, or '-').
// The suggestFn parameter allows providing custom suggestions for invalid input.
func LetDigHyp(suggestFn SuggestionFunc) Rule {
	return Alternative(
		LetDig(nil),
		Terminal("-")).
		WithSuggestionFunc(suggestFn)
}

// LDHStr returns a rule that validates a string consisting of one or more alphanumeric or hyphen characters.
// The suggestFn parameter allows providing custom suggestions for invalid input.
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
