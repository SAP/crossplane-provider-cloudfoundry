/*
Copyright 2023 SAP SE
*/

package clients

import (
	"strings"
	"sync"

	"github.com/cloudfoundry/go-cfclient/v3/config"
)

// cfConfigCache caches authenticated go-cfclient configs per credential bundle.
//
// GetCredentialConfig runs on every Connect (i.e. every reconcile) for every CF
// resource, and config.New performs an eager password-grant login against the CF
// UAA. Reference resolvers (space/org/domain ResolveByName) build a client again
// within the same reconcile, so a single reconcile can trigger several logins.
// Concurrent per-identity logins are the documented lockout trigger for the shared
// technical user, so we reuse one authenticated config per credential instead of
// logging in every reconcile. Because every controller funnels through
// GetCredentialConfig, one authenticated config (and its reused oauth2 token
// source) is shared across all CF resource types for the same credential.
//
// The lock is held across config.New (an eager login) so a concurrent stampede for
// one credential produces a single login. No TTL: the token source refreshes
// internally on expiry. Credential rotation changes the key, leaving the old entry
// until process restart.
//
// The key intentionally omits Passcode: config.New only uses UserPassword, and a
// passcode is a one-time code that must not be cached. If passcode auth is ever
// wired into GetCredentialConfig, it must bypass this cache.
var (
	cfCacheMu     sync.Mutex
	cfConfigCache = map[string]*config.Config{}
)

func cachedCFConfig(url, email, password string) (*config.Config, error) {
	key := strings.Join([]string{url, email, password}, "\x00")

	cfCacheMu.Lock()
	defer cfCacheMu.Unlock()

	if c, ok := cfConfigCache[key]; ok {
		return c, nil
	}

	cfg, err := config.New(url, config.UserPassword(email, password), config.SkipTLSValidation())
	if err != nil {
		// Do not cache a failed login; the next reconcile retries (controller-runtime
		// already applies exponential backoff to failing reconciles).
		return nil, err
	}
	cfConfigCache[key] = cfg
	return cfg, nil
}
