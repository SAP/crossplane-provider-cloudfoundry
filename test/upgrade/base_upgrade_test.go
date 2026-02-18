//go:build upgrade

package upgrade

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/test"
	upgrade "github.com/crossplane-contrib/xp-testing/pkg/upgrade"
	"github.com/crossplane-contrib/xp-testing/pkg/xpenvfuncs"
	"k8s.io/client-go/features"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

func (c *CustomUpgradeTest) Build(t *testing.T) features.Feature {
	klog.V(2).Infof("Starting upgrade test from %s to %s", c.GetFromVersion(), c.GetToVersion())
	klog.V(2).Infof("Testing resources in directories: %v", resourceDirectories)
	// Collect time metrics for upgrade tests
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		klog.V(2).Infof("Upgrade test completed in %v", duration)
	}()
	upgradeTest := upgrade.UpgradeTest{
		ProviderName:        providerName,
		ClusterName:         kindClusterName,
		FromProviderPackage: c.GetFromVersion(),
		ToProviderPackage:   c.GetToVersion(),
		ResourceDirectories: resourceDirectories,
	}

	upgradeFeature := features.New(fmt.Sprintf("Upgrade %s from %s to %s", providerName, c.GetFromVersion(), c.GetToVersion())).
		WithSetup(
			"Install provider with version "+c.GetFromVersion(),
			upgrade.ApplyProvider(upgradeTest.ClusterName, upgradeTest.FromProviderInstallOptions()),
		).
		WithSetup(
			"Import Resources for upgrade test",
			upgrade.ImportResources(upgradeTest.ResourceDirectories),
		).
		Assess(
			"Verify Resources before upgrade",
			upgrade.VerifyResources(upgradeTest.ResourceDirectories, verifyTimeout),
		)

	// Add custom pre-upgrade assessments
	for name, assessmentFunc := range c.GetPreUpgradeAssessments() {
		assessment := assessmentFunc // Capture for closure
		assessmentName := name
		upgradeFeature = upgradeFeature.Assess(
			fmt.Sprintf("Pre-upgrade: %s", assessmentName),
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				klog.V(4).Infof("Running pre-upgrade assessment: %s", assessmentName)
				return assessment(ctx, t, cfg)
			},
		)
	}

	upgradeFeature = upgradeFeature.Assess(
		"Upgrade provider to "+c.GetToVersion(),
		upgrade.UpgradeProvider(upgrade.UpgradeProviderOptions{
			ClusterName: upgradeTest.ClusterName,
			ProviderOptions: xpenvfuncs.InstallCrossplaneProviderOptions{
				Name:    providerName,
				Package: upgradeTest.ToProviderPackage,
			},
			ResourceDirectories: upgradeTest.ResourceDirectories,
			WaitForPause:        waitForPause,
		}))

	// Standard post-upgrade verification
	upgradeFeature = upgradeFeature.Assess(
		"Verify Resources after upgrade",
		upgrade.VerifyResources(upgradeTest.ResourceDirectories, verifyTimeout),
	)

	// Add custom post-upgrade assessments
	for name, assessmentFunc := range c.GetPostUpgradeAssessments() {
		assessment := assessmentFunc // Capture for closure
		assessmentName := name
		upgradeFeature = upgradeFeature.Assess(
			fmt.Sprintf("Post-upgrade: %s", assessmentName),
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				klog.V(4).Infof("Running post-upgrade assessment: %s", assessmentName)
				return assessment(ctx, t, cfg)
			},
		)
	}

	// Teardown phase
	upgradeFeature = upgradeFeature.
		WithTeardown(
			"Cleanup Resources after upgrade test",
			func(ctx context.Context, t *testing.T, envConfig *envconf.Config) context.Context {
				err := test.DeleteResourcesFromDirsGracefully(ctx, envConfig, upgradeTest.ResourceDirectories, wait.WithTimeout(verifyTimeout))
				if err != nil {
					t.Logf("Failed to delete resources during teardown: %v", err)
				}
				return ctx
			},
		).
		WithTeardown(
			"Delete Provider",
			upgrade.DeleteProvider(upgradeTest.ProviderName),
		)

	return upgradeFeature

}

func (c *CustomUpgradeTest) Run(t *testing.T, cfg *envconf.Config) {
	feature := c.Build(t)
	testenv.Test(t, feature.Feature())
}
