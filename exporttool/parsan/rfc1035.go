package parsan

// suggestConstRuneUnless returns a SuggestionFunc that suggests replacing the first
// character of the input with a constant rune, unless that character matches the
// exception.
//
// The returned function behaves as follows:
//   - Returns nil if the input is empty
//   - Returns nil if the first character equals the exception rune
//   - Otherwise, returns a single suggestion that replaces the first character
//     with the suggested rune
//
// This is useful for replacing invalid characters with a default value while
// allowing certain characters (like separators) to be handled by other rules.
func suggestConstRuneUnless(suggested, exception rune) SuggestionFunc {
	return func(in string) []*parseResult {
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
		return []*parseResult{
			{
				consumedText: string(suggested),
				rest:         remaining,
			},
		}
	}
}

// suggestConstStringsIf returns a SuggestionFunc that suggests multiple replacement
// strings when the first character of the input matches a specific character.
//
// The returned function behaves as follows:
//   - Returns nil if the input is empty
//   - Returns nil if the first character does not equal the expected rune
//   - Otherwise, returns one suggestion for each string in the suggested slice,
//     each consuming the first character and replacing it with the suggested string
//
// This enables the parser to explore multiple sanitization alternatives. For example,
// an '@' character might be replaced with either "-at-" or "-", allowing both
// possibilities to be evaluated.
func suggestConstStringsIf(suggested []string, expected rune) SuggestionFunc {
	return func(in string) []*parseResult {
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
		checked := make([]*parseResult, len(suggested))
		for i, s := range suggested {
			checked[i] = &parseResult{
				consumedText: s,
				rest:         remaining,
			}
		}
		return checked
	}
}

// RFC1035Label returns a Rule that validates and sanitizes DNS labels according
// to RFC 1035 Section 2.3.1.
//
// The grammar for a label is:
//
//	<label> ::= <letter> [ [ <ldh-str> ] <let-dig> ]
//
// Valid labels must:
//   - Start with a letter (a-z, A-Z)
//   - End with a letter or digit (if longer than one character)
//   - Contain only letters, digits, or hyphens in between
//   - Be at most 63 characters long
//
// Sanitization strategies:
//   - Invalid first character: prepended or replaced with 'x'
//   - Invalid last character: replaced with 'x'
//   - Invalid middle characters: handled by the provided suggestFn
func RFC1035Label(suggestFn SuggestionFunc) Rule {
	return Concat(
		Letter(PrependOrReplaceFirstRuneWithStrings("x")),
		Opt(Concat(
			Opt(LDHStr(suggestFn)),
			LetDig(ReplaceFirstRuneWithStrings("x"))),
		),
	).WithMaxLength(63)
}

// RFC1035LabelRelaxed returns a Rule similar to RFC1035Label but allows labels
// to start with a digit in addition to letters. This relaxation is common in
// practice, as many systems accept labels beginning with digits despite the
// strict RFC 1035 grammar.
func RFC1035LabelRelaxed(suggestFn SuggestionFunc) Rule {
	return Concat(
		LetDig(PrependOrReplaceFirstRuneWithStrings("x")),
		Opt(Concat(
			Opt(LDHStr(suggestFn)),
			LetDig(ReplaceFirstRuneWithStrings("x"))),
		),
	).WithMaxLength(63)
}

// RFC1035LowerLabel returns a Rule identical to RFC1035Label but restricts
// letters to lowercase only (a-z). This is useful when case-normalized labels
// are required, enabling case-insensitive comparisons via exact string matching.
func RFC1035LowerLabel(suggestFn SuggestionFunc) Rule {
	return Concat(
		LowerLetter(PrependOrReplaceFirstRuneWithStrings("x")),
		Opt(Concat(
			Opt(LowerLDHStr(suggestFn)),
			LowerLetDig(ReplaceFirstRuneWithStrings("x"))),
		),
	).WithMaxLength(63)
}

// RFC1035LowerLabelRelaxed returns a Rule that combines the relaxed starting
// character requirement (allowing digits) with lowercase letter enforcement.
// Labels may start with a lowercase letter or digit and contain only lowercase
// letters, digits, and hyphens.
func RFC1035LowerLabelRelaxed(suggestFn SuggestionFunc) Rule {
	return Concat(
		LowerLetDig(PrependOrReplaceFirstRuneWithStrings("x")),
		Opt(Concat(
			Opt(LowerLDHStr(suggestFn)),
			LowerLetDig(ReplaceFirstRuneWithStrings("x"))),
		),
	).WithMaxLength(63)
}

// subdomainSuggestFn is a SuggestionFunc that handles invalid characters within
// subdomain labels by combining two strategies:
//
//  1. The '@' character is offered two replacement options: "-at-" or "-"
//  2. All other invalid characters (except '.') are replaced with '-'
//
// The '.' character is explicitly excluded because it serves as the label
// separator in subdomains and must be processed by the subdomain rule's
// concatenation logic rather than being replaced.
var subdomainSuggestFn = MergeSuggestionFuncs(
	suggestConstStringsIf([]string{"-at-", "-"}, '@'),
	suggestConstRuneUnless('-', '.'),
)

// RFC1035Subdomain is a Rule that validates and sanitizes DNS subdomains
// according to RFC 1035 Section 2.3.1.
//
// The grammar for a subdomain is:
//
//	<subdomain> ::= <label> | <subdomain> "." <label>
//
// A subdomain consists of one or more dot-separated labels, where each label
// conforms to RFC1035Label. The total length must not exceed 253 characters.
//
// Invalid characters within labels are sanitized as follows:
//   - '@' is replaced with "-at-" or "-" (both options explored)
//   - Other invalid characters are replaced with '-'
var RFC1035Subdomain = Named("rfc1035-subdomain",
	Alternative(
		RFC1035Label(subdomainSuggestFn),
		Concat(
			RFC1035Label(subdomainSuggestFn),
			Terminal("."),
			Ref("rfc1035-subdomain"),
		),
	)).WithMaxLength(253)

// RFC1035SubdomainRelaxed is a Rule similar to RFC1035Subdomain but uses
// RFC1035LabelRelaxed for each label, allowing labels to start with digits.
// This accommodates the common practice of using digit-prefixed labels in
// DNS names.
var RFC1035SubdomainRelaxed = Named("rfc1035-subdomain-relaxed",
	Alternative(
		RFC1035LabelRelaxed(subdomainSuggestFn),
		Concat(
			RFC1035LabelRelaxed(subdomainSuggestFn),
			Terminal("."),
			Ref("rfc1035-subdomain-relaxed"),
		),
	)).WithMaxLength(253)

// RFC1035LowerSubdomain is a Rule identical to RFC1035Subdomain but enforces
// lowercase letters throughout all labels. This produces case-normalized
// subdomain strings suitable for systems that perform case-insensitive DNS
// comparisons via exact string matching.
var RFC1035LowerSubdomain = Named("rfc1035-lower-subdomain",
	Alternative(
		RFC1035LowerLabel(subdomainSuggestFn),
		Concat(
			RFC1035LowerLabel(subdomainSuggestFn),
			Terminal("."),
			Ref("rfc1035-lower-subdomain"),
		),
	)).WithMaxLength(253)

// RFC1035LowerSubdomainRelaxed is a Rule that combines relaxed starting
// character requirements with lowercase letter enforcement. Labels may
// start with a lowercase letter or digit, and all letters are restricted
// to lowercase. This is the most permissive variant while still enforcing
// case normalization.
var RFC1035LowerSubdomainRelaxed = Named("rfc1035-lower-subdomain-relaxed",
	Alternative(
		RFC1035LowerLabelRelaxed(subdomainSuggestFn),
		Concat(
			RFC1035LowerLabelRelaxed(subdomainSuggestFn),
			Terminal("."),
			Ref("rfc1035-lower-subdomain-relaxed"),
		),
	)).WithMaxLength(253)
