package servicecredentialbinding

import (
	"strings"

	"github.com/google/uuid"
)

// IsCanonicalGUID reports whether value uses the 36-character hyphenated UUID
// form accepted by force rotation. Hexadecimal letters may use either case;
// whitespace, braces, URNs, and unhyphenated UUIDs are not accepted.
func IsCanonicalGUID(value string) bool {
	if len(value) != 36 || strings.TrimSpace(value) != value {
		return false
	}
	_, err := uuid.Parse(value)
	return err == nil && strings.Count(value, "-") == 4
}

// SameGUID compares canonical GUIDs case-insensitively, while preserving exact
// equality for legacy identities. It does not validate a force-rotation target;
// callers must use IsCanonicalGUID before accepting a new request.
func SameGUID(a, b string) bool {
	return a == b || IsCanonicalGUID(a) && IsCanonicalGUID(b) && strings.EqualFold(a, b)
}
