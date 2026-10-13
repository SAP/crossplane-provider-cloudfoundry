package metadata

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/utils/ptr"
)

func TestStripDefaultTags(t *testing.T) {
	cases := map[string]struct {
		in   map[string]*string
		want map[string]*string
	}{
		"nil": {in: nil, want: nil},
		"only default tags": {
			in: map[string]*string{
				"crossplane-kind":           ptr.To("app.cloudfoundry.crossplane.io"),
				"crossplane-name":           ptr.To("old-cr"),
				"crossplane-providerconfig": ptr.To("old-pc"),
			},
			want: nil,
		},
		"mixed keeps user keys": {
			in: map[string]*string{
				"crossplane-name": ptr.To("old-cr"),
				"note":            ptr.To("keep"),
			},
			want: map[string]*string{"note": ptr.To("keep")},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, StripDefaultTags(tc.in)); diff != "" {
				t.Errorf("StripDefaultTags() -want +got:\n%s", diff)
			}
		})
	}
}
