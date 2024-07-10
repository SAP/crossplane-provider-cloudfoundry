package http

import "net/http"

// ClientProvider encapsulates basic functionality of a cf client
type ClientProvider interface {
	// Client returns a *http.Client
	Client(followRedirects bool) (*http.Client, error)

	// ReAuthenticate tells the provider to re-initialize the auth context
	ReAuthenticate() error
}

// UnauthenticatedClientProvider a ClientProvider which explicitly does run unauthenticated requests
type UnauthenticatedClientProvider struct {
	httpClient               *http.Client
	httpClientNonRedirecting *http.Client
}

// Client returns a *http.Client
func (c *UnauthenticatedClientProvider) Client(followRedirects bool) (*http.Client, error) {
	if followRedirects {
		return c.httpClient, nil
	}
	return c.httpClientNonRedirecting, nil
}

// ReAuthenticate no-op with this type
func (c *UnauthenticatedClientProvider) ReAuthenticate() error {
	return nil
}

// NewUnauthenticatedClientProvider factory using standard http.Client
func NewUnauthenticatedClientProvider(httpClient *http.Client) *UnauthenticatedClientProvider {
	client := &http.Client{
		Transport:     httpClient.Transport,
		Timeout:       httpClient.Timeout,
		Jar:           httpClient.Jar,
		CheckRedirect: httpClient.CheckRedirect,
	}
	clientNonRedirecting := &http.Client{
		Transport: httpClient.Transport,
		Timeout:   httpClient.Timeout,
		Jar:       httpClient.Jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return &UnauthenticatedClientProvider{
		httpClient:               client,
		httpClientNonRedirecting: clientNonRedirecting,
	}
}
