package utils

import (
	"fmt"

	"github.com/fatih/color"
)

var (
	fieldColor     = color.New(color.FgHiGreen).SprintFunc()
	attributeColor = color.New(color.FgHiMagenta).SprintFunc()
)

func PrintLine(field string, value string, width int) {
	// Format field and value with alignment
	fieldFormatted := fmt.Sprintf("%-*s", width, field+":")
	valueFormatted := fmt.Sprint(value)
	fmt.Println(fieldColor(fieldFormatted) + attributeColor(valueFormatted))
}
