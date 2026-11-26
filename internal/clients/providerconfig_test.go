/*
Copyright 2026 SAP SE
*/

package clients

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
)

const (
	testPCName       = "pc"
	testNamespace    = "crossplane-system"
	testSecretName   = "cf-credentials"
	testCredsKey     = "credentials"
	testEndpointKey  = "endpoint"
	testCredentials  = `{"email":"user@example.com","password":"secret"}`
	tlsHintSubstring = "spec.skipTLSValidation"
)

// newFakeCF starts an HTTPS server with a self-signed certificate that answers
// the two calls config.New makes: API root discovery and the UAA password grant.
// If failRoot is set, the root endpoint returns 500 to produce a non-TLS error.
func newFakeCF(t *testing.T, failRoot bool) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/" && failRoot:
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"errors":[{"code":10001,"title":"CF-ServerError","detail":"boom"}]}`)
		case r.URL.Path == "/":
			fmt.Fprintf(w, `{"links":{"self":{"href":%q},"uaa":{"href":%q},"login":{"href":%q}}}`, srv.URL, srv.URL, srv.URL)
		case r.URL.Path == "/oauth/token":
			fmt.Fprint(w, `{"access_token":"access","refresh_token":"refresh","token_type":"bearer","expires_in":3600}`)
		default:
			t.Logf("fake CF: unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestKube(t *testing.T, pc *v1beta1.ProviderConfig, endpoint string) *k8sfake.ClientBuilder {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, v1beta1.SchemeBuilder.AddToScheme(scheme))

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: testSecretName, Namespace: testNamespace},
		Data: map[string][]byte{
			testCredsKey:    []byte(testCredentials),
			testEndpointKey: []byte(endpoint),
		},
	}
	return k8sfake.NewClientBuilder().WithScheme(scheme).WithObjects(pc, secret)
}

func secretSelector(key string) xpv2.CommonCredentialSelectors {
	return xpv2.CommonCredentialSelectors{
		SecretRef: &xpv2.SecretKeySelector{
			SecretReference: xpv2.SecretReference{Name: testSecretName, Namespace: testNamespace},
			Key:             key,
		},
	}
}

func TestGetCredentialConfig(t *testing.T) {
	type want struct {
		err     bool
		certErr bool
		tlsHint bool
	}

	cases := map[string]struct {
		skipTLS          bool
		endpointInSecret bool
		failRoot         bool
		want             want
	}{
		"DefaultVerifiesTLS": {
			skipTLS: false,
			want:    want{err: true, certErr: true, tlsHint: true},
		},
		"SkipTLSValidationEnabled": {
			skipTLS: true,
			want:    want{},
		},
		"EndpointFromSecretAlsoVerified": {
			skipTLS:          false,
			endpointInSecret: true,
			want:             want{err: true, certErr: true, tlsHint: true},
		},
		"NonTLSErrorNotWrapped": {
			skipTLS:  true,
			failRoot: true,
			want:     want{err: true},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srv := newFakeCF(t, tc.failRoot)

			pc := &v1beta1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{Name: testPCName},
				Spec: v1beta1.ProviderConfigSpec{
					Credentials: v1beta1.ProviderCredentials{
						Source:                    xpv2.CredentialsSourceSecret,
						CommonCredentialSelectors: secretSelector(testCredsKey),
					},
					SkipTLSValidation: tc.skipTLS,
				},
			}
			if tc.endpointInSecret {
				pc.Spec.Endpoint = &v1beta1.EndpointConfig{
					Source:                    xpv2.CredentialsSourceSecret,
					CommonCredentialSelectors: secretSelector(testEndpointKey),
				}
			} else {
				url := srv.URL
				pc.Spec.APIEndpoint = &url
			}

			kube := newTestKube(t, pc, srv.URL).Build()

			mg := &v1alpha1.Space{}
			mg.SetProviderConfigReference(&xpv2.Reference{Name: testPCName})

			cfg, err := GetCredentialConfig(context.Background(), kube, mg)

			if !tc.want.err {
				require.NoError(t, err)
				assert.NotNil(t, cfg)
				return
			}

			require.Error(t, err)
			assert.Nil(t, cfg)

			var certErr *tls.CertificateVerificationError
			assert.Equal(t, tc.want.certErr, errors.As(err, &certErr),
				"tls.CertificateVerificationError in chain: %v", err)
			assert.Equal(t, tc.want.tlsHint, strings.Contains(err.Error(), tlsHintSubstring),
				"skipTLSValidation hint in error: %v", err)
			if tc.want.tlsHint {
				assert.Contains(t, err.Error(), fmt.Sprintf("%q", testPCName))
			}
		})
	}
}
