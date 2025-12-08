package main

import (
	"fmt"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/parsan"
	"github.com/charmbracelet/huh"
)

var inputText = ""

func updateFn(rule parsan.Rule) func() string {
	return func() string {
		suggestions := parsan.ParseAndSanitize(inputText, rule)
		if len(suggestions) == 0 {
			return "input cannot be sanitized"
		}
		return suggestions[0]
		// s := &strings.Builder{}
		// for _, suggestion := range suggestions {
		// 	fmt.Fprintf(s, " - %s\n", suggestion)
		// }
		// return s.String()
	}
}

func main() {
	input := huh.NewInput().
		Title("Enter a string to convert").
		Value(&inputText)
	rfc1035subdomain := huh.NewNote().Title("RFC1035 Subdomain").
		DescriptionFunc(updateFn(parsan.RFC1035Subdomain), &inputText)
	// rfc1035Label := huh.NewNote().Title("RFC1035 Label").
	// 	DescriptionFunc(updateFn(parsan.RFC1035Label(parsan.SuggestConstRune('-'))), &inputText)
	rfc1035LowerSubdomain := huh.NewNote().Title("RFC1035 Lowercase Subdomain").
		DescriptionFunc(updateFn(parsan.RFC1035LowerSubdomain), &inputText)
	// rfc1035LowerLabel := huh.NewNote().Title("RFC1035 Lower-case Label").
	// 	DescriptionFunc(updateFn(parsan.RFC1035LowerLabel(parsan.SuggestConstRune('-'))), &inputText)
	rfc1035LowerSubdomainRelaxed := huh.NewNote().Title("RFC1035 Lowercase Subdomain (relaxed)").
		DescriptionFunc(updateFn(parsan.RFC1035LowerSubdomainRelaxed), &inputText)
	group := huh.NewGroup(input, rfc1035subdomain, rfc1035LowerSubdomain, rfc1035LowerSubdomainRelaxed)
	form := huh.NewForm(group)
	err := form.Run()
	if err != nil {
		fmt.Println(err)
	}
}
