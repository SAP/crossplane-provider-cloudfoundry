package parsan

// suggestConstRuneUnless creates a SuggestionFunc that replaces invalid characters
// with a constant rune, except when the character matches the exception.
//
// When the first character of the input equals the exception rune, no suggestion
// is made (returns nil). Otherwise, the character is replaced with the suggested
// rune and parsing continues with the remainder of the input.
//
// Parameters:
//   - suggested: the rune to substitute for invalid characters
//   - exception: a rune that should not be replaced (typically handled elsewhere)
//
// Returns nil if the input is empty or the first character is the exception.
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

// suggestConstStringsIf creates a SuggestionFunc that suggests multiple replacement
// strings when a specific character is encountered.
//
// This is useful when an invalid character could be meaningfully replaced by
// different alternatives. For example, "@" might be replaced with "-at-" or "-".
//
// Parameters:
//   - suggested: a slice of possible replacement strings to try
//   - expected: the character that triggers these suggestions
//
// Returns nil if the input is empty or the first character doesn't match expected.
// Otherwise, returns one result per suggested string, allowing the parser to
// explore multiple sanitization paths.
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

// RFC1035Label creates a rule that validates and sanitizes DNS labels according
// to RFC 1035 Section 2.3.1:
//
//	<label> ::= <letter> [ [ <ldh-str> ] <let-dig> ]
//
// A valid label must:
//   - Start with a letter (a-z, A-Z)
//   - End with a letter or digit (if more than one character)
//   - Contain only letters, digits, or hyphens in the middle
//
// Sanitization behavior:
//   - Invalid first character: prepended or replaced with 'x'
//   - Invalid last character: replaced with 'x'
//   - Invalid middle characters: handled by the provided suggestFn
//
// Parameters:
//   - suggestFn: determines how to sanitize invalid characters in the middle of the label
func RFC1035Label(suggestFn SuggestionFunc) Rule {
	return Concat(
		Letter(PrependOrReplaceFirstRuneWithStrings("x")),
		Opt(Concat(
			Opt(LDHStr(suggestFn)),
			LetDig(ReplaceFirstRuneWithStrings("x"))),
		),
	)
}

// RFC1035LowerLabel is identical to RFC1035Label but enforces lowercase letters.
// This is useful for systems that require case-insensitive DNS label comparison
// to be performed via exact string matching.
func RFC1035LowerLabel(suggestFn SuggestionFunc) Rule {
	return Concat(
		LowerLetter(PrependOrReplaceFirstRuneWithStrings("x")),
		Opt(Concat(
			Opt(LowerLDHStr(suggestFn)),
			LowerLetDig(ReplaceFirstRuneWithStrings("x"))),
		),
	)
}

// subdomainSuggestFn handles invalid characters within subdomain labels by
// merging two suggestion strategies:
//   - '@' characters are replaced with either "-at-" or "-" (providing alternatives)
//   - All other invalid characters (except '.') are replaced with '-'
//
// The '.' character is excluded because it serves as the label separator
// in subdomains and must be handled by the subdomain rule itself.
var subdomainSuggestFn = MergeSuggestionFuncs(
	suggestConstStringsIf([]string{"-at-", "-"}, '@'),
	suggestConstRuneUnless('-', '.'),
)

// RFC1035Subdomain validates and sanitizes DNS subdomains according to
// RFC 1035 Section 2.3.1:
//
//	<subdomain> ::= <label> | <subdomain> "." <label>
//
// A subdomain consists of one or more labels separated by dots, where each
// label follows the RFC1035Label format. The total length is constrained
// to 63 characters maximum.
//
// Sanitization behavior for invalid characters within labels:
//   - '@' may be replaced with "-at-" or "-"
//   - Other invalid characters are replaced with '-'
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

// RFC1035LowerSubdomain is identical to RFC1035Subdomain but enforces lowercase
// letters throughout. This ensures consistent case-normalized subdomain strings
// suitable for case-insensitive comparisons via exact string matching.
var RFC1035LowerSubdomain = RuleWithLengthConstraint(Named("rfc1035-lower-subdomain",
	Alternative(
		RFC1035LowerLabel(subdomainSuggestFn),
		Concat(
			RFC1035LowerLabel(subdomainSuggestFn),
			Terminal("."),
			RefNamed("rfc1035-lower-subdomain"),
		),
	),
), 63)
