package parsan

func suggestConstRuneUnless(suggested, exception rune) SuggestionFunc {
	return func(in string) []*Checked {
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
		return []*Checked{
			{
				Suggestion: string(suggested),
				ToParse:    remaining,
			},
		}
	}
}

func suggestConstStringsIf(suggested []string, expected rune) SuggestionFunc {
	return func(in string) []*Checked {
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
		checked := make([]*Checked, len(suggested))
		for i, s := range suggested {
			checked[i] = &Checked{
				Suggestion: s,
				ToParse:    remaining,
			}
		}
		return checked
	}
}

func RFC1035Label(suggestFn SuggestionFunc) Type {
	return Concat(
		Letter(PrependFirstRuneWithStrings("x")),
		Opt(Concat(
			Opt(LDHStr(suggestFn)),
			LetDig(ReplaceFirstRuneWithStrings("x"))),
		),
	)
}

var subdomainSuggestFn = MergeSuggestionFuncs(
	suggestConstStringsIf([]string{"-at-", "-"}, '@'),
	suggestConstRuneUnless('-', '.'),
)

type rFC1035Subdomain struct {
	Type
}

var _ RuleWithMaxLength = rFC1035Subdomain{}

func (r rFC1035Subdomain) MaxLength() int {
	return 63
}

var RFC1035Subdomain = rFC1035Subdomain{Named("rfc1035-subdomain",
	Alternative(
		RFC1035Label(subdomainSuggestFn),
		Concat(
			RFC1035Label(subdomainSuggestFn),
			Terminal("."),
			RefNamed("rfc1035-subdomain"),
		),
	),
)}
