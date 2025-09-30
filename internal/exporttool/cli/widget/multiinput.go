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

func makeSelectOptionPair(options [][2]string) []huh.Option[string] {
	selects := make([]huh.Option[string], len(options))
	for i := range options {
		selects[i] = huh.NewOption(options[i][1], options[i][0])
	}
	return selects
}

func MultiInput(title string, options []string) []string {
	selected := []string{}
	huh.NewMultiSelect[string]().
		Options(
			makeSelectOption(options)...
		).
		Title(title).
		Value(&selected).
		Run()
	return selected
}

func MultiInputPair(title string, options [][2]string) []string {
	selected := []string{}
	huh.NewMultiSelect[string]().
		Options(
			makeSelectOptionPair(options)...
		).
		Title(title).
		Value(&selected).
		Run()
	return selected
}
