package clients

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestIsValidGUID(t *testing.T) {
	cases := map[string]struct {
		guid  string
		valid bool
	}{
		"valid GUID": {
			guid:  "33fd5b0b-4f3b-4b1b-8b3d-3b5f7b4b3b4b",
			valid: true,
		},
		"invalid GUID": {
			guid:  "not-a-valid-guid",
			valid: false,
		},
		"empty string": {
			guid:  "",
			valid: false,
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			result := IsValidGUID(tc.guid)
			if diff := cmp.Diff(tc.valid, result); diff != "" {
				t.Errorf("IsValidGUID(...): -want, +got:\n%s", diff)
			}
		})
	}
}
