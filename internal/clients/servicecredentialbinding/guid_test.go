package servicecredentialbinding

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const testCanonicalGUID = "abcdef01-2345-4678-9abc-def012345678"

func TestIsCanonicalGUID(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  bool
	}{
		{name: "lowercase", value: testCanonicalGUID, want: true},
		{name: "uppercase", value: strings.ToUpper(testCanonicalGUID), want: true},
		{name: "mixed case", value: "ABCdef01-2345-4678-9aBC-def012345678", want: true},
		{name: "empty"},
		{name: "boolean", value: "true"},
		{name: "timestamp", value: "2026-10-28T10:00:00Z"},
		{name: "leading whitespace", value: " " + testCanonicalGUID},
		{name: "trailing whitespace", value: testCanonicalGUID + " "},
		{name: "braces", value: "{" + testCanonicalGUID + "}"},
		{name: "URN", value: "urn:uuid:" + testCanonicalGUID},
		{name: "unhyphenated", value: strings.ReplaceAll(testCanonicalGUID, "-", "")},
		{name: "invalid hexadecimal", value: "gbcdef01-2345-4678-9abc-def012345678"},
		{name: "misplaced hyphen", value: "abcdef0-12345-4678-9abc-def012345678"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, IsCanonicalGUID(tc.value))
		})
	}
}

func TestSameGUID(t *testing.T) {
	for _, tc := range []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{name: "identical canonical GUIDs", a: testCanonicalGUID, b: testCanonicalGUID, want: true},
		{name: "case insensitive canonical GUIDs", a: testCanonicalGUID, b: strings.ToUpper(testCanonicalGUID), want: true},
		{name: "different GUIDs", a: testCanonicalGUID, b: rotationTestK0},
		{name: "braced alias is not equivalent", a: testCanonicalGUID, b: "{" + testCanonicalGUID + "}"},
		{name: "unhyphenated alias is not equivalent", a: testCanonicalGUID, b: strings.ReplaceAll(testCanonicalGUID, "-", "")},
		{name: "whitespace is not normalized", a: testCanonicalGUID, b: testCanonicalGUID + " "},
		{name: "identical legacy identities", a: "legacy-binding", b: "legacy-binding", want: true},
		{name: "legacy identities remain case sensitive", a: "legacy-binding", b: "LEGACY-BINDING"},
		{name: "identical empty identities", want: true},
		{name: "one empty identity", a: testCanonicalGUID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, SameGUID(tc.a, tc.b))
			require.Equal(t, tc.want, SameGUID(tc.b, tc.a), "comparison must be symmetric")
		})
	}
}
