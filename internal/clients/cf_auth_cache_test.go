/*
Copyright 2023 SAP SE
*/

package clients

// Proves the CF login-reuse cache: cachedCFConfig reuses a single UAA password
// login per credential across reconciles, refreshing via the refresh_token grant
// instead of logging in again. Each reconcile still builds its own *config.Config
// from the shared token source (its own transport — no shared state to race on).
// Non-parallel: shares the package-global cfAuthCache / cfAuthSF.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	cfv3 "github.com/cloudfoundry/go-cfclient/v3/client"
)

// mintJWT builds a structurally valid JWT access token with the given expiry.
// go-cfclient's jwt.ToOAuth2Token requires three "."-separated segments and
// base64.RawURLEncoding-decodes the payload for its "exp" claim.
func mintJWT(exp time.Time) string {
	b64 := base64.RawURLEncoding.EncodeToString
	header := b64([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := b64([]byte(`{"exp":` + strconv.FormatInt(exp.Unix(), 10) + `}`))
	return header + "." + payload + ".sig"
}

// fakeCFAPI serves CF service discovery (GET /) + the UAA token endpoint,
// counting logins by grant type.
type fakeCFAPI struct {
	passwordLogins atomic.Int64
	refreshLogins  atomic.Int64

	// accessTokenExpiry is how far in the future issued access tokens expire.
	// A negative value issues already-expired tokens (forces a refresh).
	accessTokenExpiry time.Duration
	// failRefresh makes the token endpoint reject refresh_token grants.
	failRefresh bool
	// firstPasswordExpired issues an already-expired access token for the first
	// password login and a long-lived one for every subsequent login. Combined
	// with failRefresh it models a dead shared token source that a fresh
	// password re-login recovers.
	firstPasswordExpired bool
}

func (f *fakeCFAPI) start(t *testing.T) string {
	t.Helper()
	if f.accessTokenExpiry == 0 {
		f.accessTokenExpiry = time.Hour
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			_ = r.ParseForm()
			grant := r.Form.Get("grant_type")
			exp := f.accessTokenExpiry
			switch grant {
			case "refresh_token":
				if f.failRefresh {
					w.WriteHeader(http.StatusUnauthorized)
					_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_token"})
					return
				}
				f.refreshLogins.Add(1)
			default: // "password"
				n := f.passwordLogins.Add(1)
				if f.firstPasswordExpired {
					if n == 1 {
						exp = -time.Minute // first login's token is already expired
					} else {
						exp = time.Hour // re-login recovers with a long-lived token
					}
				}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				// expires_in drives oauth2's ReuseTokenSource refresh decision;
				// the JWT exp drives go-cfclient's config.Token parsing. Keep both
				// consistent.
				"access_token":  mintJWT(time.Now().Add(exp)),
				"token_type":    "bearer",
				"expires_in":    int64(exp.Seconds()),
				"refresh_token": "rt",
			})
			return
		}
		// discovery: point login + uaa back at this server
		_ = json.NewEncoder(w).Encode(map[string]any{
			"links": map[string]any{
				"login":   map[string]string{"href": "http://" + r.Host},
				"uaa":     map[string]string{"href": "http://" + r.Host},
				"app_ssh": map[string]any{"meta": map[string]string{"oauth_client": "ssh"}},
			},
		})
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func resetCFCache() {
	cfAuthCache.Range(func(k, _ any) bool {
		cfAuthCache.Delete(k)
		cfAuthSF.Forget(k.(string))
		return true
	})
}

// buildClient mirrors ClientFnBuilder: build a config from the cache, then a client.
func buildClient(url, email, password string) error {
	cfg, err := cachedCFConfig(context.Background(), url, email, password)
	if err != nil {
		return err
	}
	_, err = cfv3.New(cfg)
	return err
}

// N reconciles with the same credential reuse one login.
func TestCFCache_ReusesLogin(t *testing.T) {
	resetCFCache()
	fake := &fakeCFAPI{}
	url := fake.start(t)
	for i := 0; i < 50; i++ {
		if err := buildClient(url, "u@example.com", "pw"); err != nil {
			t.Fatalf("buildClient: %v", err)
		}
	}
	if got := fake.passwordLogins.Load(); got != 1 {
		t.Errorf("password logins across 50 reconciles = %d, want 1 (login reused)", got)
	}
	if got := fake.refreshLogins.Load(); got != 0 {
		t.Errorf("refresh logins = %d, want 0 (token still valid)", got)
	}
}

// Concurrent reconciles collapse to one login (singleflight).
func TestCFCache_ConcurrentOneLogin(t *testing.T) {
	resetCFCache()
	fake := &fakeCFAPI{}
	url := fake.start(t)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = buildClient(url, "u@example.com", "pw") }()
	}
	wg.Wait()
	if got := fake.passwordLogins.Load(); got != 1 {
		t.Errorf("password logins across 50 concurrent reconciles = %d, want 1", got)
	}
}

// Distinct credentials (different API URLs) get distinct logins.
func TestCFCache_DistinctCredsDistinctLogins(t *testing.T) {
	resetCFCache()
	f1, f2 := &fakeCFAPI{}, &fakeCFAPI{}
	u1, u2 := f1.start(t), f2.start(t)
	if err := buildClient(u1, "u@example.com", "pw"); err != nil {
		t.Fatalf("buildClient u1: %v", err)
	}
	if err := buildClient(u2, "u@example.com", "pw"); err != nil {
		t.Fatalf("buildClient u2: %v", err)
	}
	if g1, g2 := f1.passwordLogins.Load(), f2.passwordLogins.Load(); g1 != 1 || g2 != 1 {
		t.Errorf("distinct-credential logins = (%d,%d), want (1,1)", g1, g2)
	}
}

// A failed bootstrap is not cached: the next reconcile retries.
func TestCFCache_FailedLoginNotCached(t *testing.T) {
	resetCFCache()
	// No server: discovery/login fails -> error, must not be cached.
	if _, err := cachedCFConfig(context.Background(), "http://127.0.0.1:1", "u@example.com", "pw"); err == nil {
		t.Fatal("expected login error")
	}
	if _, ok := cfAuthCache.Load(cfAuthKey("http://127.0.0.1:1", "u@example.com", "pw")); ok {
		t.Error("failed login must not be cached")
	}
}

// An expired access token recovers via the refresh_token grant (not a new
// password login), and concurrent refreshes are race-free (run under -race).
func TestCFCache_ExpiredTokenRefreshesNoPasswordBurst(t *testing.T) {
	resetCFCache()
	// Password grant issues an already-expired access token so the shared token
	// source must refresh; refresh grant issues a long-lived one.
	fake := &fakeCFAPI{accessTokenExpiry: -time.Minute}
	url := fake.start(t)

	// Prime the cache (one password login) so concurrent callers hit the shared
	// ReuseTokenSource refresh path together.
	if err := buildClient(url, "u@example.com", "pw"); err != nil {
		t.Fatalf("prime buildClient: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = buildClient(url, "u@example.com", "pw") }()
	}
	wg.Wait()

	if got := fake.passwordLogins.Load(); got != 1 {
		t.Errorf("password logins = %d, want 1 (recovery must use refresh grant, not password)", got)
	}
	if got := fake.refreshLogins.Load(); got == 0 {
		t.Errorf("refresh logins = %d, want >=1 (expired token should refresh)", got)
	}
}

// A dead refresh token triggers exactly one re-bootstrap (fresh password login)
// per reconcile and does not wedge or loop.
func TestCFCache_DeadRefreshTokenRebootstraps(t *testing.T) {
	resetCFCache()
	// Expired access token forces a refresh, and refresh is rejected -> the
	// shared source is unusable, so cachedCFConfig re-bootstraps once.
	fake := &fakeCFAPI{accessTokenExpiry: -time.Minute, failRefresh: true}
	url := fake.start(t)

	_, err := cachedCFConfig(context.Background(), url, "u@example.com", "pw")
	if err == nil {
		t.Fatal("expected error: refresh is dead and re-bootstrap still yields an expired token")
	}
	// One initial bootstrap + exactly one re-bootstrap.
	if got := fake.passwordLogins.Load(); got != 2 {
		t.Errorf("password logins = %d, want 2 (initial + one re-bootstrap)", got)
	}
	// The known-bad entry must not linger.
	if _, ok := cfAuthCache.Load(cfAuthKey(url, "u@example.com", "pw")); ok {
		t.Error("unusable entry must be dropped, not wedged")
	}
}

// When the shared token source dies and many reconciles fail at once, recovery
// must coalesce to a single re-login — not one password login per caller — and
// the freshly recovered entry must not be evicted by a straggler still holding
// the dead one. This is the concurrent counterpart to the re-bootstrap path:
// CompareAndDelete (not Delete) and keeping the singleflight key are what make
// it hold. Run under -race.
func TestCFCache_ConcurrentDeadTokenSingleRelogin(t *testing.T) {
	resetCFCache()
	// First password login issues an already-expired token and the refresh grant
	// is dead, so the shared source fails for every concurrent caller at once;
	// the second password login recovers with a long-lived token.
	fake := &fakeCFAPI{failRefresh: true, firstPasswordExpired: true}
	url := fake.start(t)

	var wg sync.WaitGroup
	errs := make([]error, 50)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); errs[i] = buildClient(url, "u@example.com", "pw") }(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("reconcile %d failed; recovery should succeed for all callers: %v", i, err)
		}
	}
	// Exactly one dead bootstrap + one coalesced re-login. More would mean a
	// straggler clobbered the recovered entry and forced extra password logins.
	if got := fake.passwordLogins.Load(); got != 2 {
		t.Errorf("password logins = %d, want 2 (one dead bootstrap + one coalesced re-login)", got)
	}
}
