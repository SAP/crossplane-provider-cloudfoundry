package clients

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
)

// TestSkipTLS asserts TLS validation is skipped only when spec.skipTLSValidation
// is explicitly true. Unset (nil) and false must verify.
func TestSkipTLS(t *testing.T) {
	cases := map[string]struct {
		field *bool
		want  bool
	}{
		"Unset_VerifiesByDefault": {field: nil, want: false},
		"False_Verifies":          {field: ptr.To(false), want: false},
		"True_Skips":              {field: ptr.To(true), want: true},
	}
	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			pc := &v1beta1.ProviderConfig{
				Spec: v1beta1.ProviderConfigSpec{SkipTLSValidation: tc.field},
			}
			if got := skipTLS(pc); got != tc.want {
				t.Errorf("skipTLS() = %v, want %v", got, tc.want)
			}
		})
	}
}
