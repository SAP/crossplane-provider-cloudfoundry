package widget

import (
	"github.com/charmbracelet/huh"
)

func makeSelectOption(options []string) []huh.Option[string] {
	selects := make([]huh.Option[string], len(options))
	for i := range options {
		selects[i] = huh.NewOption(options[i], options[i])
	}
	return selects
}

func MultiInput(title string, options []string) []string {
	selected := []string{}
	if err := huh.NewMultiSelect[string]().
		Options(
			makeSelectOption(options)...,
		).
		Title(title).
		Value(&selected).
		Run(); err != nil {
		panic(err)
	}
	return selected
}
