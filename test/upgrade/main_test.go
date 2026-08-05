//go:build upgrade

//
// This file (main_test.go) contains TestMain and cluster setup logic for all upgrade tests.
// It performs the following initialization:
//   - Creates a kind cluster with Crossplane and the CloudFoundry provider
//   - Configures the provider with DeploymentRuntimeConfig for debug logging
//   - Handles both "local" (development) and registry (release) provider images
//   - Sets up CloudFoundry credentials as a Kubernetes secret
//   - Discovers and loads test resources from testdata directories
//
// The setup happens once before all tests run, ensuring consistent test environment
// across both baseline and custom upgrade tests.

package upgrade

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	testutil "github.com/SAP/crossplane-provider-cloudfoundry/test"
	"github.com/crossplane-contrib/xp-testing/pkg/images"
	"github.com/crossplane-contrib/xp-testing/pkg/setup"
	"github.com/crossplane-contrib/xp-testing/pkg/vendored"
	"github.com/vladimirvivien/gexe"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/support/kind"
)

const (
	// Crossplane version
	crossplaneVersion = "1.20.1"

	// Provider identification
	providerName = "provider-cloudfoundry"

	// Image paths
	packageBasePath           = "ghcr.io/sap/crossplane-provider-cloudfoundry/crossplane/provider-cloudfoundry"
	controllerPackageBasePath = "ghcr.io/sap/crossplane-provider-cloudfoundry/crossplane/provider-cloudfoundry-controller"

	// Secrets
	cfSecretName = "cf-credentials"

	// Test namespace
	namespacePrefix = "cf-upgrade-test-"

	// Environment variables
	cfEndpointEnvVar = "CF_ENDPOINT"
	uutImagesEnvVar  = "UUT_IMAGES"

	// Environment variables - Optional with defaults
	verifyTimeoutEnvVar = "UPGRADE_TEST_VERIFY_TIMEOUT"
	waitForPauseEnvVar  = "UPGRADE_TEST_WAIT_FOR_PAUSE"

	// Defaults for optional parameters
	defaultResourceDirectory = "./testdata/baseCrs"
	defaultVerifyTimeout     = 30
	defaultWaitForPause      = 1
)

var (
	testenv                   env.Environment
	kindClusterName           string
	resourceDirectories       []string
	ignoreResourceDirectories = []string{
		// Add any directories to ignore here, e.g.:
		// "./testdata/experimental",
	}

	// Default Values
	resourceDirectoryRoot string
	verifyTimeout         time.Duration
	waitForPause          time.Duration
)

var (
	fromTag    string
	toTag      string
	fromLayout imageLayout
	toLayout   imageLayout
)

// TestMain is the entry point for all upgrade tests
// It sets up the test environment once and runs all tests
func TestMain(m *testing.M) {
	var verbosity = 4
	testutil.SetupLogging(verbosity)

	namespace := envconf.RandomName(namespacePrefix, 16)

	SetupClusterWithCrossplane(namespace)

	os.Exit(testenv.Run(m))
}

// SetupClusterWithCrossplane creates a kind cluster with Crossplane and the CloudFoundry provider
// This is called once before all tests run
func SetupClusterWithCrossplane(namespace string) {
	testenv = env.New()

	// Get CloudFoundry credentials from environment
	cfSecretData := testutil.GetCFCredentialsOrPanic()

	// Load version tags for upgrade (FROM -> TO)
	fromTag, toTag = loadPackageTags()
	verifyTimeout = loadVerifyTimeout()
	waitForPause = loadWaitForPause()

	resourceDirectoryRoot = defaultResourceDirectory

	// Discover all resource directories
	loadResourceDirectories()
	klog.V(4).Infof("found resource directories: %s", resourceDirectories)

	// Resolve image layouts independently (handles local and registry tags).
	fromLayout, toLayout = resolveImagePaths(fromTag, toTag)

	// Pull images from registry if needed (skip if local). Controller images are
	// optional for single-image layouts.
	pullImagesIfNeeded(fromTag, toTag, fromLayout, toLayout)

	// Configure provider deployment with debug logging and faster sync
	deploymentRuntimeConfig := getDeploymentRuntimeConfig()

	// Configure cluster setup with Crossplane and provider
	cfg := setup.ClusterSetup{
		ProviderName: providerName,
		Images: images.ProviderImages{
			Package:         fromLayout.packageImage,
			ControllerImage: fromLayout.controllerImage,
		},
		CrossplaneSetup: setup.CrossplaneSetup{
			Version:  crossplaneVersion,
			Registry: setup.DockerRegistry,
		},
		DeploymentRuntimeConfig: deploymentRuntimeConfig,
	}

	// Register callback for after cluster creation
	cfg.PostCreate(func(clusterName string) env.Func {
		kindClusterName = clusterName
		return func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			klog.V(4).Infof("upgrade cluster %s has been created", clusterName)
			// xp-testing copies local package metadata into Crossplane's cache but
			// does not load the local xpkg image when ControllerImage is nil. Load
			// the package image explicitly; this is not a controller-image load.
			if err := loadLocalPackageImagesIntoKind(clusterName, fromTag, toTag, fromLayout, toLayout); err != nil {
				return ctx, fmt.Errorf("failed to load local package images into kind: %w", err)
			}
			// Load registry images into kind (xp-testing uses PackagePullPolicy: Never).
			if err := loadRegistryImagesIntoKind(clusterName, fromTag, toTag, fromLayout, toLayout); err != nil {
				return ctx, fmt.Errorf("failed to load registry images into kind: %w", err)
			}
			return ctx, nil
		}
	})

	// Configure the test environment
	_ = cfg.Configure(testenv, &kind.Cluster{})

	// Setup CloudFoundry credentials and ProviderConfig
	testenv.Setup(
		testutil.ApplySecretInCrossplaneNamespace(cfSecretName, cfSecretData),
	)
}

// resolveImagePaths determines the correct package-plus-optional-controller
// layout independently for FROM and TO versions.
func resolveImagePaths(fromTag, toTag string) (imageLayout, imageLayout) {
	isLocalFromTag := fromTag == localTagName
	isLocalToTag := toTag == localTagName

	var localPackage string
	var localController *string
	if isLocalFromTag || isLocalToTag {
		uutImages := os.Getenv(uutImagesEnvVar)
		if uutImages == "" {
			panic(fmt.Errorf("%s environment variable is required when FROM_TAG or TO_TAG is 'local'", uutImagesEnvVar))
		}

		localPackage, localController = testutil.GetImagesFromJsonOrPanic(uutImages)
		klog.V(4).Infof("Loaded local images from %s", uutImagesEnvVar)
	}

	from := resolveImageLayout(fromTag, localPackage, localController, packageBasePath, controllerPackageBasePath)
	to := resolveImageLayout(toTag, localPackage, localController, packageBasePath, controllerPackageBasePath)
	if isLocalFromTag {
		klog.V(4).Infof("Using local images for FROM: %s", from.packageImage)
	}
	if isLocalToTag {
		klog.V(4).Infof("Using local images for TO: %s", to.packageImage)
	}

	return from, to
}

// pullImagesIfNeeded pulls only the images resolved for each registry side.
func pullImagesIfNeeded(fromTag, toTag string, from, to imageLayout) {
	for _, side := range []struct {
		tag    string
		name   string
		images imageLayout
	}{
		{fromTag, "FROM", from},
		{toTag, "TO", to},
	} {
		if side.tag == localTagName {
			klog.V(4).Infof("Skipping pull for %s=local (using locally built images)", side.name)
			continue
		}
		klog.V(4).Infof("Pulling %s images: %s", side.name, side.tag)
		for _, image := range imageNames(side.images) {
			mustPullImage(image)
		}
		klog.V(4).Infof("Successfully pulled %s images: %s", side.name, side.tag)
	}
}

// loadLocalPackageImagesIntoKind loads local xpkg images into kind. The
// package image must be available because ControllerImage is nil for a
// single-image layout and xp-testing otherwise only copies its descriptor.
func loadLocalPackageImagesIntoKind(clusterName, fromTag, toTag string, from, to imageLayout) error {
	runner := gexe.New()
	for _, image := range localPackageImagesToLoad(fromTag, toTag, from, to) {
		if err := loadImageIntoKind(runner, clusterName, image); err != nil {
			return err
		}
	}
	return nil
}

// loadRegistryImagesIntoKind loads pulled registry images into the kind cluster.
// xp-testing uses PackagePullPolicy: Never, so all resolved images must be
// pre-loaded.
func loadRegistryImagesIntoKind(clusterName, fromTag, toTag string, from, to imageLayout) error {
	runner := gexe.New()
	for _, image := range registryImagesToLoad(fromTag, toTag, from, to) {
		if err := loadImageIntoKind(runner, clusterName, image); err != nil {
			return err
		}
	}
	return nil
}

// loadImageIntoKind loads a single image into the kind cluster
func loadImageIntoKind(runner *gexe.Echo, clusterName, image string) error {
	cmd := fmt.Sprintf("kind load docker-image %s --name %s", image, clusterName)
	p := runner.RunProc(cmd)

	if p.Err() != nil {
		return fmt.Errorf("failed to load %s into kind: %w: %s", image, p.Err(), p.Result())
	}

	klog.V(4).Infof("Loaded image %s into kind", image)
	return nil
}

// getDeploymentRuntimeConfig creates a DeploymentRuntimeConfig with debug logging and faster sync
// TODO: Consider extracting to shared package - identical in BTP and CF providers
func getDeploymentRuntimeConfig() *vendored.DeploymentRuntimeConfig {
	return &vendored.DeploymentRuntimeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cf-provider-runtime-config",
		},
		Spec: vendored.DeploymentRuntimeConfigSpec{
			DeploymentTemplate: &vendored.DeploymentTemplate{
				Spec: &appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{},
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "package-runtime",
									Args: []string{"--debug", "--sync=10s"},
								},
							},
						},
					},
				},
			},
		},
	}
}

// loadPackageTags loads the FROM and TO version tags from environment variables
func loadPackageTags() (string, string) {
	fromTagVar := os.Getenv("UPGRADE_TEST_FROM_TAG")
	if fromTagVar == "" {
		panic("UPGRADE_TEST_FROM_TAG environment variable is required")
	}

	toTagVar := os.Getenv("UPGRADE_TEST_TO_TAG")
	if toTagVar == "" {
		panic("UPGRADE_TEST_TO_TAG environment variable is required")
	}

	return fromTagVar, toTagVar
}

// loadResourceDirectories discovers all directories containing YAML test files
func loadResourceDirectories() {
	directories, err := loadDirectoriesWithYAMLFiles(resourceDirectoryRoot)
	if err != nil {
		panic(fmt.Errorf("failed to read resource directories from %s: %w", resourceDirectoryRoot, err))
	}

	resourceDirectories = directories
}

// loadDirectoriesWithYAMLFiles recursively finds directories containing .yaml files
// TODO: Consider extracting to shared package - identical in BTP and CF providers
func loadDirectoriesWithYAMLFiles(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read resource files from %s: %w", path, err)
	}

	var directories []string
	containsYAMLFile := false

	for _, entry := range entries {
		if entry.IsDir() {
			// Skip ignored directories
			if !slices.Contains(ignoreResourceDirectories, filepath.Join(path, entry.Name())) {
				subEntries, err := loadDirectoriesWithYAMLFiles(filepath.Join(path, entry.Name()))
				if err != nil {
					return nil, err
				}

				directories = append(directories, subEntries...)
			}
		} else if strings.HasSuffix(entry.Name(), ".yaml") {
			containsYAMLFile = true
		}
	}

	// Only include directories that actually contain YAML files
	if containsYAMLFile {
		directories = append(directories, path)
	}

	return directories, nil
}

// mustPullImage pulls a Docker image and panics if it fails
// TODO: Consider extracting to shared package - identical in BTP and CF providers
func mustPullImage(image string) {
	klog.Info("Pulling ", image)
	runner := gexe.New()
	p := runner.RunProc(fmt.Sprintf("docker pull %s", image))
	klog.V(4).Info(p.Out())
	if p.Err() != nil {
		panic(fmt.Errorf("docker pull %v failed: %w: %s", image, p.Err(), p.Result()))
	}
}

// loadVerifyTimeout loads the verify timeout from environment or uses default
func loadVerifyTimeout() time.Duration {
	timeoutStr := os.Getenv(verifyTimeoutEnvVar)
	if timeoutStr == "" {
		klog.V(4).Infof("Using default verify timeout: %d minutes", defaultVerifyTimeout)
		return time.Duration(defaultVerifyTimeout) * time.Minute
	}

	timeoutMin, err := strconv.Atoi(timeoutStr)
	if err != nil {
		klog.Warningf("Invalid %s value '%s', using default: %d minutes", verifyTimeoutEnvVar, timeoutStr, defaultVerifyTimeout)
		return time.Duration(defaultVerifyTimeout) * time.Minute
	}

	if timeoutMin <= 0 {
		klog.Warningf("Invalid %s value %d (must be > 0), using default: %d minutes", verifyTimeoutEnvVar, timeoutMin, defaultVerifyTimeout)
		return time.Duration(defaultVerifyTimeout) * time.Minute
	}

	klog.V(4).Infof("Using verify timeout from %s: %d minutes", verifyTimeoutEnvVar, timeoutMin)
	return time.Duration(timeoutMin) * time.Minute
}

// loadWaitForPause loads the wait for pause duration from environment or uses default
func loadWaitForPause() time.Duration {
	waitStr := os.Getenv(waitForPauseEnvVar)
	if waitStr == "" {
		klog.V(4).Infof("Using default wait for pause: %d minutes", defaultWaitForPause)
		return time.Duration(defaultWaitForPause) * time.Minute
	}

	waitMin, err := strconv.Atoi(waitStr)
	if err != nil {
		klog.Warningf("Invalid %s value '%s', using default: %d minutes", waitForPauseEnvVar, waitStr, defaultWaitForPause)
		return time.Duration(defaultWaitForPause) * time.Minute
	}

	if waitMin <= 0 {
		klog.Warningf("Invalid %s value %d (must be > 0), using default: %d minutes", waitForPauseEnvVar, waitMin, defaultWaitForPause)
		return time.Duration(defaultWaitForPause) * time.Minute
	}

	klog.V(4).Infof("Using wait for pause from %s: %d minutes", waitForPauseEnvVar, waitMin)
	return time.Duration(waitMin) * time.Minute
}
