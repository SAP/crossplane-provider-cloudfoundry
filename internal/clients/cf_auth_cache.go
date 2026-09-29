/*
Copyright 2023 SAP SE
*/

package clients

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/config"
	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"
)

// cfAuthCache reuses one CF UAA login per credential across all reconciles.
//
// GetCredentialConfig runs on every Connect (i.e. every reconcile) for every CF
// resource, and config.New(UserPassword(...)) performs an eager password-grant
// login against CF UAA. Reference resolvers (space/org/domain ResolveByName) build
// a client again within the same reconcile, so a single reconcile can trigger
// several logins. Concurrent per-identity password logins are the documented
// lockout trigger for the shared technical user.
//
// We cache the *login* — a goroutine-safe oauth2.TokenSource — not the
// *config.Config. A config carries a mutable, shared transport: on a 401
// go-cfclient's retryableAuthTransport re-runs the password grant and reassigns
// oauth2.Transport.Source without synchronization, so sharing one config across
// controllers races and can stampede logins. Instead every reconcile builds its
// own short-lived config from the shared token source, so each has its own
// transport, and token expiry refreshes via the refresh_token grant — never the
// password grant.
//
// The key intentionally omits Passcode: only UserPassword is used, and a passcode
// is a one-time code that must not be cached. If passcode auth is ever wired into
// GetCredentialConfig it must bypass this cache.

// cfAuthEntry is the cached, reusable login for one credential.
type cfAuthEntry struct {
	src      oauth2.TokenSource // oauth2.ReuseTokenSource: goroutine-safe, refreshes via refresh_token
	loginURL string             // discovered CF login endpoint
	uaaURL   string             // discovered CF UAA endpoint
}

var (
	cfAuthCache sync.Map // map[string]*cfAuthEntry
	cfAuthSF    singleflight.Group
)

// bootstrapTimeout bounds the one-time discovery+login per credential.
const bootstrapTimeout = 30 * time.Second

// cfAuthKey identifies a credential bundle. url+email+password only (see doc above).
func cfAuthKey(url, email, password string) string {
	return strings.Join([]string{url, email, password}, "\x00")
}

// cachedCFConfig returns a go-cfclient config for the credential, reusing a single
// UAA login across reconciles. Each call returns a fresh *config.Config seeded with
// the current token (grant type refresh_token, so config.New performs no login) and
// the pre-discovered auth URLs (so config.New performs no discovery GET). The build
// is therefore fully local.
func cachedCFConfig(url, email, password string) (*config.Config, error) {
	key := cfAuthKey(url, email, password)

	entry, err := getOrBootstrap(key, url, email, password)
	if err != nil {
		return nil, err
	}

	tok, err := entry.src.Token()
	if err != nil {
		// The refresh token is dead/revoked (or bootstrap produced a token that
		// can no longer refresh). Drop the wedged entry and bootstrap once more
		// with a fresh password login, then retry a single time.
		cfAuthCache.Delete(key)
		cfAuthSF.Forget(key)
		entry, err = getOrBootstrap(key, url, email, password)
		if err != nil {
			return nil, err
		}
		tok, err = entry.src.Token()
		if err != nil {
			// Still unusable: drop the entry so the next reconcile starts clean
			// rather than reusing a known-bad login.
			cfAuthCache.Delete(key)
			return nil, err
		}
	}

	return config.New(url,
		config.Token(tok.AccessToken, tok.RefreshToken),
		config.AuthTokenURL(entry.loginURL, entry.uaaURL),
		config.SkipTLSValidation(),
	)
}

// getOrBootstrap returns the cached entry for key, creating it (once, coalesced
// across concurrent callers) via a single password login if absent.
func getOrBootstrap(key, url, email, password string) (*cfAuthEntry, error) {
	if e, ok := cfAuthCache.Load(key); ok {
		return e.(*cfAuthEntry), nil
	}

	v, err, _ := cfAuthSF.Do(key, func() (interface{}, error) {
		// Another caller may have populated the cache while we queued.
		if e, ok := cfAuthCache.Load(key); ok {
			return e.(*cfAuthEntry), nil
		}
		entry, err := bootstrapCFAuth(url, email, password)
		if err != nil {
			// Do not cache a failed login; the next reconcile retries
			// (controller-runtime already backs off failing reconciles).
			return nil, err
		}
		cfAuthCache.Store(key, entry)
		return entry, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*cfAuthEntry), nil
}

// bootstrapCFAuth performs the one login per credential: discover the CF auth
// endpoints, then obtain a refreshing token source via the password grant.
func bootstrapCFAuth(url, email, password string) (*cfAuthEntry, error) {
	httpClient := &http.Client{
		Timeout: bootstrapTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // mirrors config.SkipTLSValidation()
		},
	}

	loginURL, uaaURL, err := discoverCFAuthEndpoints(httpClient, url)
	if err != nil {
		return nil, err
	}

	oauthCfg := &oauth2.Config{
		ClientID: "cf",
		Endpoint: oauth2.Endpoint{
			AuthURL:   loginURL + "/oauth/auth",
			TokenURL:  uaaURL + "/oauth/token",
			AuthStyle: oauth2.AuthStyleInHeader,
		},
	}

	// context.Background(), NOT a reconcile ctx: ReuseTokenSource captures this
	// context for every future refresh, so it must outlive any single reconcile.
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)

	tok, err := oauthCfg.PasswordCredentialsToken(ctx, email, password)
	if err != nil {
		return nil, fmt.Errorf("cloudfoundry UAA login failed: %w", err)
	}

	return &cfAuthEntry{
		src:      oauthCfg.TokenSource(ctx, tok),
		loginURL: loginURL,
		uaaURL:   uaaURL,
	}, nil
}

// discoverCFAuthEndpoints reads the CF API root and returns the login and UAA
// endpoints — the same discovery go-cfclient performs internally.
func discoverCFAuthEndpoints(httpClient *http.Client, url string) (loginURL, uaaURL string, err error) {
	root := strings.TrimRight(url, "/") + "/"
	resp, err := httpClient.Get(root)
	if err != nil {
		return "", "", fmt.Errorf("error while discovering CF auth endpoints: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	var body struct {
		Links struct {
			Login struct {
				Href string `json:"href"`
			} `json:"login"`
			Uaa struct {
				Href string `json:"href"`
			} `json:"uaa"`
		} `json:"links"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", "", fmt.Errorf("error decoding CF API root: %w", err)
	}
	if body.Links.Login.Href == "" || body.Links.Uaa.Href == "" {
		return "", "", fmt.Errorf("CF API root did not advertise login/uaa endpoints")
	}
	return strings.TrimRight(body.Links.Login.Href, "/"), strings.TrimRight(body.Links.Uaa.Href, "/"), nil
}
