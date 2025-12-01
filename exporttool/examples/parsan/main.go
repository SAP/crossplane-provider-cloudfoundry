package main

import (
	"fmt"
	"strings"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/parsan"
	"github.com/charmbracelet/huh"
)

var inputText = ""

func updateRFC1035Subdomain() string {
	suggestions := parsan.ParseAndSanitize(inputText, parsan.RFC1035Subdomain)
	if len(suggestions) == 0 {
		return "input cannot be sanitized"
	}
	s := &strings.Builder{}
	for _, suggestion := range suggestions {
		fmt.Fprintf(s, " - %s\n", suggestion)
	}
	return s.String()
}

func updateRFC1035Label() string {
	suggestions := parsan.ParseAndSanitize(inputText, parsan.RFC1035Label(parsan.SuggestConstRune('-')))
	if len(suggestions) == 0 {
		return "input cannot be sanitized"
	}
	s := &strings.Builder{}
	for _, suggestion := range suggestions {
		fmt.Fprintf(s, " - %s\n", suggestion)
	}
	return s.String()
}

func main() {
	input := huh.NewInput().
		Title("Enter a string to convert").
		Value(&inputText)
	rfc1035subdomain := huh.NewNote().Title("RFC1035 Subdomain").
		DescriptionFunc(updateRFC1035Subdomain, &inputText)
	rfc1035Label := huh.NewNote().Title("RFC1035 Subdomain").
		DescriptionFunc(updateRFC1035Label, &inputText)
	group := huh.NewGroup(input, rfc1035subdomain, rfc1035Label)
	form := huh.NewForm(group)
	err := form.Run()
	if err != nil {
		fmt.Println(err)
	}
}
