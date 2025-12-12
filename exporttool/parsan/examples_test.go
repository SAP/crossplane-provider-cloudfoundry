package parsan_test

import (
	"fmt"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/parsan"
)

func ExampleTerminal() {
	rule := parsan.Terminal("match")
	suggestions := parsan.ParseAndSanitize("match", rule)
	fmt.Print(suggestions[0])
	//output: match
}

func ExampleTerminal_no_match() {
	rule := parsan.Terminal("match")
	suggestions := parsan.ParseAndSanitize("invalid", rule)
	fmt.Print(len(suggestions))
	//output: 0
}

func ExampleRange() {
	rule := parsan.Range('a', 'z')
	suggestions := parsan.ParseAndSanitize("e", rule)
	fmt.Print(suggestions[0])
	//output: e
}

func ExampleRange_no_match() {
	rule := parsan.Range('a', 'z')
	suggestions := parsan.ParseAndSanitize("A", rule)
	fmt.Print(len(suggestions))
	//output: 0
}

func ExampleRange_with_suggestions() {
	rule := parsan.Range('a', 'z').WithSuggestionFunc(parsan.ReplaceFirstRuneWithStrings("x"))
	suggestions := parsan.ParseAndSanitize("A", rule)
	fmt.Print(suggestions[0])
	//output: x
}

func ExampleConcat() {
	rule := parsan.Concat(parsan.Range('0', '9'), parsan.Range('a', 'z'))
	suggestions := parsan.ParseAndSanitize("2b", rule)
	fmt.Print(suggestions[0])
	//output: 2b
}

func ExampleConcat_no_match() {
	rule := parsan.Concat(parsan.Range('0', '9'), parsan.Range('a', 'z'))
	suggestions := parsan.ParseAndSanitize("b2", rule)
	fmt.Print(len(suggestions))
	//output: 0
}

func ExampleAlternative() {
	rule := parsan.Alternative(
		parsan.Terminal("either"),
		parsan.Terminal("or"),
	)
	suggestions := parsan.ParseAndSanitize("either", rule)
	fmt.Print(suggestions[0])
	//output: either
}

func ExampleAlternative_no_match() {
	rule := parsan.Alternative(
		parsan.Terminal("either"),
		parsan.Terminal("or"),
	)
	suggestions := parsan.ParseAndSanitize("none", rule)
	fmt.Print(len(suggestions))
	//output: 0
}

func ExampleNamed() {
	// rule := a | a <rule>
	rule := parsan.Named("rule",
		parsan.Alternative(
			parsan.Terminal("a"),
			parsan.Concat(
				parsan.Terminal("a"),
				parsan.Ref("rule"),
			),
		),
	)
	suggestions := parsan.ParseAndSanitize("aaaaaaaaa", rule)
	fmt.Print(suggestions[0])
	//output: aaaaaaaaa
}

func ExampleSeq() {
	rule := parsan.Seq(2, 4, parsan.Terminal("a"))
	suggestions := parsan.ParseAndSanitize("aaa", rule)
	fmt.Print(suggestions[0])
	//output: aaa
}

func ExampleSeq_no_match() {
	rule := parsan.Seq(2, 4, parsan.Terminal("a"))
	suggestions := parsan.ParseAndSanitize("a", rule)
	fmt.Print(len(suggestions))
	//output: 0
}

func ExampleLDHStr() {
	suggestions := parsan.ParseAndSanitize("this-is-1-valid-LDHStr", parsan.LDHStr(nil))
	fmt.Print(suggestions[0])
	//output: this-is-1-valid-LDHStr
}

func ExampleLDHStr_no_match() {
	suggestions := parsan.ParseAndSanitize("inva!id", parsan.LDHStr(nil))
	fmt.Print(len(suggestions))
	//output: 0
}

func ExampleLDHStr_sanitize() {
	suggestions := parsan.ParseAndSanitize("inva!id", parsan.LDHStr(parsan.SuggestConstRune('-')))
	fmt.Print(suggestions[0])
	//output: inva-id
}

func ExampleRFC1035Label() {
	suggestions := parsan.ParseAndSanitize("this-is-a-valid-RFC1035-label", parsan.RFC1035Label(nil))
	fmt.Print(suggestions[0])
	//output: this-is-a-valid-RFC1035-label
}

func ExampleRFC1035Label_no_match() {
	suggestions := parsan.ParseAndSanitize("0this-is-an-!nvalid-RFC1035-label#", parsan.RFC1035Label(nil))
	fmt.Print(len(suggestions))
	//output: 0
}

func ExampleRFC1035Label_sanitize() {
	suggestions := parsan.ParseAndSanitize("0this-is-an-!nvalid-RFC1035-label#",
		parsan.RFC1035Label(parsan.SuggestConstRune('A')))
	fmt.Print(suggestions[0])
	//output: x0this-is-an-Anvalid-RFC1035-labelx
}
