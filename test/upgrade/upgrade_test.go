//go:build upgrade

package upgrade

import (
	"context"
	"fmt"
	"testing"

	"github.com/SAP/crossplane-provider-cloudfoundry/test"
	upgrade "github.com/crossplane-contrib/xp-testing/pkg/upgrade"
	"github.com/crossplane-contrib/xp-testing/pkg/xpenvfuncs"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestUpgradeProvider(t *testing.T) {
	klog.V(2).Infof("Starting upgrade test from %s to %s", fromTag, toTag)
	klog.V(2).Infof("Testing resources in directories: %v", resourceDirectories)

	upgradeTest := upgrade.UpgradeTest{
		ProviderName:        providerName,
		ClusterName:         kindClusterName,
		FromProviderPackage: fromPackage,
		ToProviderPackage:   toPackage,
		ResourceDirectories: resourceDirectories,
	}
	upgradeFeature := features.New(fmt.Sprintf("Upgrade %s from %s to %s", providerName, fromTag, toTag)).
		WithSetup(
			"Install provider with version "+fromTag,
			upgrade.ApplyProvider(upgradeTest.ClusterName, upgradeTest.FromProviderInstallOptions()),
		).
		WithSetup(
			"Import Resources for upgrade test",
			upgrade.ImportResources(upgradeTest.ResourceDirectories),
		).
		Assess(
			"Verify Resources before upgrade",
			upgrade.VerifyResources(upgradeTest.ResourceDirectories, verifyTimeout),
		).
		Assess(
			"Upgrade provider to "+toTag,
			upgrade.UpgradeProvider(upgrade.UpgradeProviderOptions{
				ClusterName: upgradeTest.ClusterName,
				ProviderOptions: xpenvfuncs.InstallCrossplaneProviderOptions{
					Name:    providerName,
					Package: upgradeTest.ToProviderPackage,
				},
				ResourceDirectories: upgradeTest.ResourceDirectories,
				WaitForPause:        waitForPause,
			})).
		Assess(
			"Verify Resources after upgrade",
			upgrade.VerifyResources(upgradeTest.ResourceDirectories, verifyTimeout),
		).
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

	testenv.Test(t, upgradeFeature.Feature())

}
