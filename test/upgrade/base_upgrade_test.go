//go:build upgrade

// TestUpgradeProvider is the baseline upgrade test that verifies the provider can be
// successfully upgraded from one version to another while maintaining resource health.
//
// This test demonstrates the use of the CustomUpgradeTestBuilder framework with
// default baseline behavior. The test flow is:
//  1. Install provider at the "from" version
//  2. Import test resources from baseline directories
//  3. Verify all resources are healthy
//  4. Upgrade provider to the "to" version
//  5. Verify all resources remain healthy after upgrade
//  6. Clean up resources and provider

package upgrade

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/crossplane-contrib/xp-testing/pkg/resources"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

func TestUpgradeProvider(t *testing.T) {
	fromTag, toTag := loadTags()

	serviceInstanceDir := filepath.Join(resourceDirectoryRoot, "serviceInstance")
	serviceCredentialBindingDir := filepath.Join(resourceDirectoryRoot, "serviceCredentialBinding")
	serviceInstanceDependencyChain := []string{serviceInstanceDir, serviceCredentialBindingDir}

	domainDir := filepath.Join(resourceDirectoryRoot, "domain")
	routeDir := filepath.Join(resourceDirectoryRoot, "route")
	appDir := filepath.Join(resourceDirectoryRoot, "app")
	domainDependencyChain := []string{domainDir, routeDir, appDir}

	dependencyDirs := append(serviceInstanceDependencyChain, domainDependencyChain...)
	requiredDirs := removeFromDirs(resourceDirectories, dependencyDirs)

	upgradeTest := NewCustomUpgradeTest("baseline-upgrade-test").
		FromVersion(fromTag).
		ToVersion(toTag).
		WithResourceDirectories(resourceDirectories).
		SkipDefaultResourceVerification().
		WithCustomPreUpgradeAssessment(
			"Check all required resources are healthy before upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				return verifyResources(ctx, t, cfg, requiredDirs, verifyTimeout)
			},
		).
		WithCustomPreUpgradeAssessment(
			"Check service instance and service credential binding resources are healthy before upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				return verifyDependencyChain(ctx, t, cfg, serviceInstanceDependencyChain, verifyTimeout)
			},
		).
		WithCustomPreUpgradeAssessment(
			"Check domain, route, and app resources are healthy before upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				return verifyDependencyChain(ctx, t, cfg, domainDependencyChain, verifyTimeout)
			},
		).
		WithCustomPostUpgradeAssessment(
			"Check all required resources are healthy after upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				return verifyResources(ctx, t, cfg, requiredDirs, verifyTimeout)
			},
		).
		WithCustomPostUpgradeAssessment(
			"Check service instance and service credential binding resources are healthy after upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				return verifyDependencyChain(ctx, t, cfg, serviceInstanceDependencyChain, verifyTimeout)
			},
		).
		WithCustomPostUpgradeAssessment(
			"Check domain, route, and app resources are healthy after upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				return verifyDependencyChain(ctx, t, cfg, domainDependencyChain, verifyTimeout)
			},
		)
	testenv.Test(t, upgradeTest.Feature())
}

// verifyResources waits for resources in dirs to be ready
func verifyResources(ctx context.Context, t *testing.T, cfg *envconf.Config, dirs []string, timeout time.Duration) context.Context {
	for _, dir := range dirs {
		klog.V(4).Infof("verify resources of directory %s", dir)
		if err := resources.WaitForResourcesToBeSynced(ctx, cfg, dir, nil, wait.WithTimeout(timeout)); err != nil {
			t.Errorf("verify resources of directory %s failed: %v", dir, err)
		}
	}

	return ctx
}

// verifyDependencyChain verifies the resource directories in the order as they appear in the slice
// and skips the remaining if any resource directory in the chain fails verification
func verifyDependencyChain(ctx context.Context, t *testing.T, cfg *envconf.Config, dirs []string, timeout time.Duration) context.Context {
	for i, dir := range dirs {
		klog.V(4).Infof("verify resources of directory %s", dir)
		if err := resources.WaitForResourcesToBeSynced(ctx, cfg, dir, nil, wait.WithTimeout(timeout)); err != nil {
			t.Errorf("verify resources of directory %s failed: %v — skipping verification of remaining dependent directories: %s", dir, err, strings.Join(dirs[i+1:], ", "))
			return ctx
		}
	}
	return ctx
}

// removeFromDirs is a helper function to remove directories from the list of directories to verify,
// allowing dependent resources to be verified separately
func removeFromDirs(dirs []string, remove []string) []string {
	result := slices.Clone(dirs)
	return slices.DeleteFunc(result, func(d string) bool {
		return slices.Contains(remove, d)
	})
}

// loadTags is a helper function to load FROM and TO tags for tests
// This allows custom tests to reuse the same version configuration
func loadTags() (string, string) {
	return fromTag, toTag
}
