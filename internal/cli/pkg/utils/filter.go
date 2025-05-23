package utils

import (
	"regexp"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	// error messages
	errCompileRegex = "Could not compile Regex"
)

func IsFullMatch(pattern, input string) (bool) {
    re, err := regexp.Compile("^" + pattern + "$")
	kingpin.FatalIfError(err, "%s", errCompileRegex)

    return re.MatchString(input)
}

// truncates all characters that are not allowed by RFC1123
func NormalizeToRFC1123(input string) string {
	rfc1123Regex := `^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`

	re, err := regexp.Compile(rfc1123Regex)
	kingpin.FatalIfError(err, "%s", errCompileRegex)

	// Check if the entire input string matches the pattern
	if re.MatchString(input) {
		return input
	}

	// Truncate characters from the input string until it matches the pattern
	validString := ""
	for _, char := range input {
		tempString := validString + string(char)
		if re.MatchString(tempString) {
			validString = tempString
		}
	}

	return validString
}