/*
Copyright 2023 SAP SE
*/

package clients

// Proves the CF login-reuse cache: GetCredentialConfig builds a go-cfclient config
// on every reconcile (config.New = an eager CF UAA login). Cache the authenticated
// config per credential so repeated reconciles across all CF resource types reuse
// one login. Non-parallel: shares the package-global cfConfigCache.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	cfv3 "github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"
)

// fakeCFAPI serves CF service discovery (GET /) + the UAA token endpoint,
// counting logins (token POSTs).
type fakeCFAPI struct {
	logins atomic.Int64
}

func (f *fakeCFAPI) start(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			f.logins.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "at", "token_type": "bearer", "expires_in": 3600, "refresh_token": "rt",
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
	cfCacheMu.Lock()
	cfConfigCache = map[string]*config.Config{}
	cfCacheMu.Unlock()
}

// buildClient mirrors ClientFnBuilder: cache the config, then build a client from it.
func buildClient(url, email, password string) error {
	cfg, err := cachedCFConfig(url, email, password)
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
	if got := fake.logins.Load(); got != 1 {
		t.Errorf("CF logins across 50 reconciles = %d, want 1 (config cached)", got)
	}
}

// Concurrent reconciles collapse to one login.
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
	if got := fake.logins.Load(); got != 1 {
		t.Errorf("CF logins across 50 concurrent reconciles = %d, want 1", got)
	}
}

// Distinct credentials (different API URLs) get distinct cached configs.
func TestCFCache_DistinctCredsDistinctLogins(t *testing.T) {
	resetCFCache()
	f1, f2 := &fakeCFAPI{}, &fakeCFAPI{}
	u1, u2 := f1.start(t), f2.start(t)
	_ = buildClient(u1, "u@example.com", "pw")
	_ = buildClient(u2, "u@example.com", "pw")
	if g1, g2 := f1.logins.Load(), f2.logins.Load(); g1 != 1 || g2 != 1 {
		t.Errorf("distinct-credential logins = (%d,%d), want (1,1)", g1, g2)
	}
}

// A failed login is not cached: the next reconcile retries.
func TestCFCache_FailedLoginNotCached(t *testing.T) {
	resetCFCache()
	// No server: config.New cannot discover/login -> error, must not be cached.
	if _, err := cachedCFConfig("http://127.0.0.1:1", "u@example.com", "pw"); err == nil {
		t.Fatal("expected login error")
	}
	cfCacheMu.Lock()
	_, ok := cfConfigCache["http://127.0.0.1:1\x00u@example.com\x00pw"]
	cfCacheMu.Unlock()
	if ok {
		t.Error("failed login must not be cached")
	}
}
