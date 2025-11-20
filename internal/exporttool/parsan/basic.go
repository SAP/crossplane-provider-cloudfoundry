package parsan

func PrependFirstRuneWithStrings(ss ...string) SuggestionFunc {
	return func(in string) []*Checked {
		checkeds := make([]*Checked, 0, 2*len(ss))
		for _, s := range ss {
			checkeds = append(checkeds, &Checked{
				Suggestion: s,
				ToParse:    in,
			})
			if len(in) > 0 {
				checkeds = append(checkeds, &Checked{
					Suggestion: s,
					ToParse:    in[1:],
				})
			}
		}
		return checkeds
	}
}

func AppendFirstRuneWithStrings(ss ...string) SuggestionFunc {
	return func(in string) []*Checked {
		checkeds := make([]*Checked, len(ss))
		if len(in) == 0 {
			return nil
		} else {
			first := string(in[0])
			remaining := ""
			if len(in) > 1 {
				remaining = in[1:]
			}
			for i, s := range ss {
				checkeds[i] = &Checked{
					Suggestion: first + s,
					ToParse:    remaining,
				}
			}
		}
		return checkeds
	}
}

func ReplaceFirstRuneWithStrings(ss ...string) SuggestionFunc {
	return func(in string) []*Checked {
		if len(in) == 0 {
			return nil
		}
		remaining := ""
		if len(in) > 1 {
			remaining = in[1:]
		}
		checkeds := make([]*Checked, len(ss))
		for i, s := range ss {
			checkeds[i] = &Checked{
				Suggestion: s,
				ToParse:    remaining,
			}
		}
		return checkeds
	}
}

func Digit(suggestFn SuggestionFunc) Type {
	return Range('0', '9').WithSuggestionFunc(suggestFn)
}

func Letter(suggestFn SuggestionFunc) Type {
	return Alternative(
		Range('a', 'z'),
		Range('A', 'Z'),
	).WithSuggestionFunc(suggestFn)
}

func LetDig(suggestFn SuggestionFunc) Type {
	return Alternative(
		Letter(nil),
		Digit(nil),
	).WithSuggestionFunc(suggestFn)
}

func LetDigHyp(suggestFn SuggestionFunc) Type {
	return Alternative(
		LetDig(nil),
		Terminal("-")).
		WithSuggestionFunc(suggestFn)
}

func LDHStr(suggestFn SuggestionFunc) Type {
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
