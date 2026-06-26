//go:build upgrade

// Package upgrade contains the upgrade test framework and tests.
//
// This file (upgrade_test_framework.go) provides CustomUpgradeTestBuilder,
// a fluent API for creating upgrade tests with custom validations and flexible
// configuration. Used by both baseline and custom upgrade tests.

package upgrade

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/test"
	"github.com/crossplane-contrib/xp-testing/pkg/upgrade"
	"github.com/crossplane-contrib/xp-testing/pkg/xpenvfuncs"
	kubeErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	res "sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

// CustomUpgradeTestBuilder provides an API for creating custom upgrade tests.
// It allows developers to easily configure upgrade test scenarios with custom versions,
// resource directories, and test phases while minimizing boilerplate code.
//
// Example usage:
//
//	test := NewCustomUpgradeTest("my-custom-test").
//		FromVersion("v0.3.0").
//		ToVersion("v0.3.2").
//		WithResourceDirectories([]string{"./testdata/customCRs"}).
//		WithCustomPreUpgradeAssessment("Verify custom field", assessFunc).
//		Feature()
type CustomUpgradeTestBuilder struct {
	testName string

	// Version configuration
	fromTag string
	toTag   string

	// Resource configuration
	resourceDirectories []string

	// Timeout configuration
	verifyTimeout time.Duration
	waitForPause  time.Duration

	// Custom test phases
	preUpgradeAssessments  []phaseFunc
	postUpgradeAssessments []phaseFunc

	// Disable default phases
	skipDefaultResourceVerification bool
}

// phaseFunc represents a test phase function that can be added to the test feature.
type phaseFunc struct {
	description string
	fn          func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context
}

// NewCustomUpgradeTest creates a new CustomUpgradeTestBuilder with the given test name.
// The builder will use baseline defaults from environment variables unless overridden.
//
// Example:
//
//	builder := NewCustomUpgradeTest("test-external-name-migration")
func NewCustomUpgradeTest(testName string) *CustomUpgradeTestBuilder {
	return &CustomUpgradeTestBuilder{
		testName:               testName,
		resourceDirectories:    []string{},
		preUpgradeAssessments:  []phaseFunc{},
		postUpgradeAssessments: []phaseFunc{},
		waitForPause:           waitForPause,
		verifyTimeout:          verifyTimeout,
	}
}

// FromVersion sets the source version for the upgrade test.
// Can be set to "local" to use the locally built provider.
func (b *CustomUpgradeTestBuilder) FromVersion(version string) *CustomUpgradeTestBuilder {
	b.fromTag = version
	return b
}

// ToVersion sets the target version for the upgrade test.
// Can be set to "local" to use the locally built provider.
func (b *CustomUpgradeTestBuilder) ToVersion(version string) *CustomUpgradeTestBuilder {
	b.toTag = version
	return b
}

// WithResourceDirectories sets the directories containing test resources to be used in the upgrade test.
// If not set, the baseline resource directories will be used.
//
// Example:
//
//	builder.WithResourceDirectories([]string{
//	    "./testdata/customCRs/spaceExternalName",
//	})
func (b *CustomUpgradeTestBuilder) WithResourceDirectories(dirs []string) *CustomUpgradeTestBuilder {
	b.resourceDirectories = dirs
	return b
}

// WithVerifyTimeout sets the timeout duration for resource verification.
// If not set, the value from UPGRADE_TEST_VERIFY_TIMEOUT or default (30 minutes) will be used.
func (b *CustomUpgradeTestBuilder) WithVerifyTimeout(timeout time.Duration) *CustomUpgradeTestBuilder {
	b.verifyTimeout = timeout
	return b
}

// WithWaitForPause sets the duration to wait for resources to pause during upgrade.
// If not set, the value from UPGRADE_TEST_WAIT_FOR_PAUSE or default (1 minute) will be used.
func (b *CustomUpgradeTestBuilder) WithWaitForPause(duration time.Duration) *CustomUpgradeTestBuilder {
	b.waitForPause = duration
	return b
}

// WithCustomPreUpgradeAssessment adds a custom assessment phase that runs before the upgrade.
// This can be used to verify specific conditions or resource states before upgrading.
//
// Example:
//
//	builder.WithCustomPreUpgradeAssessment("Verify external names", assertFunc)
func (b *CustomUpgradeTestBuilder) WithCustomPreUpgradeAssessment(
	description string,
	fn func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context,
) *CustomUpgradeTestBuilder {
	b.preUpgradeAssessments = append(b.preUpgradeAssessments, phaseFunc{description: description, fn: fn})
	return b
}

// WithCustomPostUpgradeAssessment adds a custom assessment phase that runs after the upgrade.
// This can be used to verify migration outcomes or new behavior in the upgraded version.
//
// Example:
//
//	builder.WithCustomPostUpgradeAssessment("Verify migrated external names", assertFunc)
func (b *CustomUpgradeTestBuilder) WithCustomPostUpgradeAssessment(
	description string,
	fn func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context,
) *CustomUpgradeTestBuilder {
	b.postUpgradeAssessments = append(b.postUpgradeAssessments, phaseFunc{description: description, fn: fn})
	return b
}

// SkipDefaultResourceVerification disables the default resource verification phases.
// This means that no checks are being carried out by default before and after upgrading the provider.
// Custom verification phases can be added using WithCustomPreUpgradeAssessment.
//
// The function that would otherwise be used is upgrade.VerifyResources(upgradeTest.ResourceDirectories, verifyTimeout).
func (b *CustomUpgradeTestBuilder) SkipDefaultResourceVerification() *CustomUpgradeTestBuilder {
	b.skipDefaultResourceVerification = true
	return b
}

// Feature constructs the upgrade test feature from the builder configuration.
// It resolves all configuration values (using defaults where not explicitly set),
// builds the test phases in the correct order, and returns a features.Feature ready for execution.
//
// The test phases are executed in this order:
//  1. Provider installation
//  2. Resource import
//  3. Pre-upgrade verification (unless skipped)
//  4. Custom pre-upgrade assessments
//  5. Provider upgrade
//  6. Post-upgrade verification (unless skipped)
//  7. Custom post-upgrade assessments
//  8. Resource cleanup
//  9. Provider cleanup
//
// Example:
//
//	feature := builder.Feature()
//	testenv.Test(t, feature)
func (b *CustomUpgradeTestBuilder) Feature() features.Feature {
	if b.fromTag == "" || b.toTag == "" {
		panic("Both fromTag and toTag must be specified before building an upgrade test feature")
	}

	fromProviderPackage, toProviderPackage := loadPackages(b.fromTag, b.toTag)

	upgradeTest := upgrade.UpgradeTest{
		ProviderName:        providerName,
		ClusterName:         kindClusterName,
		FromProviderPackage: fromProviderPackage,
		ToProviderPackage:   toProviderPackage,
		ResourceDirectories: b.resourceDirectories,
	}

	featureName := fmt.Sprintf("%s: Upgrade %s from %s to %s", b.testName, providerName, b.fromTag, b.toTag)
	feature := features.New(featureName).
		WithSetup(
			"Install provider with version "+b.fromTag,
			upgrade.ApplyProvider(upgradeTest.ClusterName, upgradeTest.FromProviderInstallOptions()),
		).
		WithSetup(
			"Apply ProviderConfig",
			getProviderConfigSetupFunc(),
		).
		WithSetup(
			"Import resources from directories",
			upgrade.ImportResources(upgradeTest.ResourceDirectories),
		)

	if !b.skipDefaultResourceVerification {
		feature = feature.Assess(
			"Verify resources before upgrade",
			upgrade.VerifyResources(upgradeTest.ResourceDirectories, b.verifyTimeout),
		)
	}

	// Add custom pre-upgrade assessments
	for _, phase := range b.preUpgradeAssessments {
		feature = feature.Assess(phase.description, phase.fn)
	}

	feature = feature.Assess(
		"Upgrade provider to version "+b.toTag,
		upgrade.UpgradeProvider(upgrade.UpgradeProviderOptions{
			ClusterName: upgradeTest.ClusterName,
			ProviderOptions: xpenvfuncs.InstallCrossplaneProviderOptions{
				Name:    providerName,
				Package: upgradeTest.ToProviderPackage,
			},
			ResourceDirectories: upgradeTest.ResourceDirectories,
			WaitForPause:        b.waitForPause,
		}),
	)

	if !b.skipDefaultResourceVerification {
		feature = feature.Assess(
			"Verify resources after upgrade",
			upgrade.VerifyResources(upgradeTest.ResourceDirectories, b.verifyTimeout),
		)
	}

	// Add custom post-upgrade assessments
	for _, phase := range b.postUpgradeAssessments {
		feature = feature.Assess(phase.description, phase.fn)
	}

	feature = feature.WithTeardown(
		"Delete resources",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			err := test.DeleteResourcesFromDirsGracefully(
				ctx,
				cfg,
				b.resourceDirectories,
				wait.WithTimeout(b.verifyTimeout),
			)
			if err != nil {
				t.Logf("failed to clean up resources: %v", err)
			}
			return ctx
		},
	).WithTeardown(
		"Delete ProviderConfig",
		getProviderConfigTeardownFunc(),
	).WithTeardown(
		"Delete provider",
		upgrade.DeleteProvider(upgradeTest.ProviderName),
	)

	return feature.Feature()
}

func loadPackages(fromTag, toTag string) (string, string) {
	return test.LoadUpgradePackages(
		fromTag, toTag,
		fromPackage, toPackage,
	)
}

// Helper to get ProviderConfig setup function
func getProviderConfigSetupFunc() func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		cfEndpoint := os.Getenv(cfEndpointEnvVar)
		if cfEndpoint == "" {
			t.Fatalf("CF_ENDPOINT environment variable is required")
		}

		err := test.CreateProviderConfig(ctx, cfg, cfg.Namespace(), cfEndpoint, cfSecretName)
		if err != nil {
			t.Fatalf("failed to create ProviderConfig: %v", err)
		}
		return ctx
	}
}

// Helper to get ProviderConfig teardown function
func getProviderConfigTeardownFunc() func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		// Delete ProviderConfig
		r, err := res.New(cfg.Client().RESTConfig())
		if err != nil {
			t.Logf("failed to create resources client: %v", err)
			return ctx
		}

		err = v1beta1.SchemeBuilder.AddToScheme(r.GetScheme()) // ← Changed from metaApi
		if err != nil {
			t.Logf("failed to add scheme: %v", err)
			return ctx
		}

		// Create the ProviderConfig object to delete
		// (just need name, rest doesn't matter for deletion)
		obj := &v1beta1.ProviderConfig{ // ← Changed from cloudfoundryv1beta1
			ObjectMeta: metav1.ObjectMeta{
				Name: "default",
			},
		}

		err = r.Delete(ctx, obj)
		if err != nil && !kubeErrors.IsNotFound(err) {
			t.Logf("failed to delete ProviderConfig: %v", err)
		}

		return ctx
	}
}
