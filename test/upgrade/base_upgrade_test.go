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
	dependentDirs := []string{serviceCredentialBindingDir}

	requiredDirs := removeFromDirs(resourceDirectories, dependentDirs)

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
			"Check service instance and dependent resources are healthy before upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				return verifyServiceInstanceWithDependents(ctx, t, cfg, serviceInstanceDir, dependentDirs, verifyTimeout)
			},
		).
		WithCustomPostUpgradeAssessment(
			"Check all required resources are healthy after upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				return verifyResources(ctx, t, cfg, requiredDirs, verifyTimeout)
			},
		).
		WithCustomPostUpgradeAssessment(
			"Check service instance and dependent resources are healthy after upgrade",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				return verifyServiceInstanceWithDependents(ctx, t, cfg, serviceInstanceDir, dependentDirs, verifyTimeout)
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

// verifyServiceInstanceWithDependents verifies the service instance directory first and
// if successful dependent directories
func verifyServiceInstanceWithDependents(ctx context.Context, t *testing.T, cfg *envconf.Config, serviceInstanceDir string, dependentDirs []string, timeout time.Duration) context.Context {
	klog.V(4).Infof("verify service instance")
	if err := resources.WaitForResourcesToBeSynced(ctx, cfg, serviceInstanceDir, nil, wait.WithTimeout(timeout)); err != nil {
		t.Errorf("verify service instance failed: %v — skipping verification of: %s", err, strings.Join(dependentDirs, ", "))
		return ctx
	}
	return verifyResources(ctx, t, cfg, dependentDirs, timeout)
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
