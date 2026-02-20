//go:build upgrade

package upgrade

import (
	"testing"
)

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
func TestUpgradeProvider(t *testing.T) {
	fromTag, toTag := loadTags()

	upgradeTest := NewCustomUpgradeTest("baseline-upgrade-test").
		FromVersion(fromTag).
		ToVersion(toTag).
		WithResourceDirectories(resourceDirectories)

	testenv.Test(t, upgradeTest.Feature())
}

// loadTags is a helper function to load FROM and TO tags for tests
// This allows custom tests to reuse the same version configuration
func loadTags() (string, string) {
	return fromTag, toTag
}
