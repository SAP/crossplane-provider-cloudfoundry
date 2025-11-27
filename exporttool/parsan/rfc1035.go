package parsan

// suggestConstRuneUnless returns a SuggestionFunc that suggests a constant rune
// for the first character of the input string, unless that character matches
// the exception rune. If the input is empty or the first character is the
// exception, returns nil. Otherwise, returns a result with the suggested rune
// and the remaining input after the first character.
func suggestConstRuneUnless(suggested, exception rune) SuggestionFunc {
	return func(in string) []*result {
		if len(in) == 0 {
			return nil
		}
		first := rune(in[0])
		if first == exception {
			return nil
		}
		remaining := ""
		if len(in) > 1 {
			remaining = in[1:]
		}
		return []*result{
			{
				sanitized: string(suggested),
				toParse:   remaining,
			},
		}
	}
}

// suggestConstStringsIf returns a SuggestionFunc that suggests multiple constant
// strings when the first character of the input matches the expected rune.
// If the input is empty or the first character doesn't match expected, returns nil.
// Otherwise, returns results for each suggested string with the remaining input
// after the first character.
func suggestConstStringsIf(suggested []string, expected rune) SuggestionFunc {
	return func(in string) []*result {
		if len(in) == 0 {
			return nil
		}
		first := rune(in[0])
		if first != expected {
			return nil
		}
		remaining := ""
		if len(in) > 1 {
			remaining = in[1:]
		}
		checked := make([]*result, len(suggested))
		for i, s := range suggested {
			checked[i] = &result{
				sanitized: s,
				toParse:   remaining,
			}
		}
		return checked
	}
}

// RFC1035Label returns a rule that validates according to the label
// definition of RFC1035.
//
//	<label> ::= <letter> [ [ <ldh-str> ] <let-dig> ]
//
// If the first letter is invalid it is prepended or replaced with the
// character 'x'.
//
// If the last letter is invalid, it is replaced with the character
// 'x'.
//
// If any interim characters are invalid, they are treated according
// to the provided suggestFn parameter.
func RFC1035Label(suggestFn SuggestionFunc) Rule {
	return Concat(
		Letter(PrependOrReplaceFirstRuneWithStrings("x")),
		Opt(Concat(
			Opt(LDHStr(suggestFn)),
			LetDig(ReplaceFirstRuneWithStrings("x"))),
		),
	)
}

// subdomainSuggestFn is a merged suggestion function that handles common
// invalid characters in subdomain labels. It suggests "-at-" or "-" for
// "@" characters, and suggests "-" for any character except ".".
var subdomainSuggestFn = MergeSuggestionFuncs(
	suggestConstStringsIf([]string{"-at-", "-"}, '@'),
	suggestConstRuneUnless('-', '.'),
)

// RFC1035Subdomain is a rule that validates according to the
// subdomain definition of RFC1035.
//
//	<subdomain> ::= <label> | <subdomain> "." <label>
//
// If the label contains an invalid interim character, the "-"
// character is suggested. For the invalid "@" character, the "-at-"
// string is also attempted. The rule enforces a maximum length
// constraint of 63 characters.
var RFC1035Subdomain = RuleWithLengthConstraint(Named("rfc1035-subdomain",
	Alternative(
		RFC1035Label(subdomainSuggestFn),
		Concat(
			RFC1035Label(subdomainSuggestFn),
			Terminal("."),
			RefNamed("rfc1035-subdomain"),
		),
	),
), 63)
