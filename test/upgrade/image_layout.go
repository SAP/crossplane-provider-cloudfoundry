package upgrade

import (
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/crossplane-contrib/xp-testing/pkg/xpenvfuncs"
)

// firstSingleImageVersion is the first Cloud Foundry release using the
// single-image package layout. Repository tag contents show v1.0.0 retaining
// a separate controller and v1.0.1 using the single-image layout. Non-semver
// registry tags are treated as current single-image builds, rather than using
// a failed controller pull to detect the layout.
const (
	localTagName            = "local"
	firstSingleImageVersion = "1.0.1"
)

// firstSingleImageSemver is parsed once at init so invalid cutoffs fail
// immediately rather than on first layout resolution.
var firstSingleImageSemver = mustParseSemver(firstSingleImageVersion)

type imageLayout struct {
	packageImage    string
	controllerImage *string
}

func resolveImageLayout(tag, localPackage string, localController *string, packageRepository, controllerRepository string) imageLayout {
	if tag == localTagName {
		return imageLayout{packageImage: localPackage, controllerImage: localController}
	}

	packageImage := fmt.Sprintf("%s:%s", packageRepository, tag)
	if isSingleImageVersion(tag) {
		return imageLayout{packageImage: packageImage}
	}

	controllerImage := fmt.Sprintf("%s:%s", controllerRepository, tag)
	return imageLayout{packageImage: packageImage, controllerImage: &controllerImage}
}

// isSingleImageVersion uses the documented v1.0.1 layout cutoff. Non-semver
// tags (for example commit or channel tags) are assumed to refer to the
// current single-image build; layout detection never depends on a failed pull.
func isSingleImageVersion(tag string) bool {
	version, err := semver.Parse(strings.TrimPrefix(tag, "v"))
	if err != nil {
		return true
	}
	return version.GTE(firstSingleImageSemver)
}

func mustParseSemver(version string) semver.Version {
	v, err := semver.Parse(version)
	if err != nil {
		panic(fmt.Errorf("invalid single-image version cutoff %q: %w", version, err))
	}
	return v
}

func providerInstallOptions(providerName string, layout imageLayout) xpenvfuncs.InstallCrossplaneProviderOptions {
	return xpenvfuncs.InstallCrossplaneProviderOptions{
		Name:            providerName,
		Package:         layout.packageImage,
		ControllerImage: layout.controllerImage,
	}
}

func imageNames(layout imageLayout) []string {
	images := []string{layout.packageImage}
	if layout.controllerImage != nil {
		images = append(images, *layout.controllerImage)
	}
	return images
}

func localPackageImagesToLoad(fromTag, toTag string, from, to imageLayout) []string {
	var images []string
	if fromTag == localTagName {
		images = append(images, from.packageImage)
	}
	if toTag == localTagName && (fromTag != localTagName || to.packageImage != from.packageImage) {
		images = append(images, to.packageImage)
	}
	return images
}

// registryImagesToLoad returns the exact registry images that must be loaded.
// The TO side is skipped when its tag equals FROM, preserving the existing
// no-duplicate load behavior.
func registryImagesToLoad(fromTag, toTag string, from, to imageLayout) []string {
	var images []string
	if fromTag != localTagName {
		images = append(images, imageNames(from)...)
	}
	if toTag != localTagName && toTag != fromTag {
		images = append(images, imageNames(to)...)
	}
	return images
}
