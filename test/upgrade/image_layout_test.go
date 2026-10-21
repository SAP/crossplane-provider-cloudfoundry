package upgrade

import (
	"reflect"
	"testing"
)

func TestResolveImageLayout(t *testing.T) {
	localController := "local/controller:latest"
	tests := map[string]struct {
		tag               string
		localPackage      string
		localController   *string
		wantPackage       string
		wantController    *string
		packageRepository string
		controllerRepo    string
	}{
		"local single image": {
			tag:               localTagName,
			localPackage:      "local/provider-xpkg:latest",
			wantPackage:       "local/provider-xpkg:latest",
			packageRepository: "registry/provider",
			controllerRepo:    "registry/controller",
		},
		"local two images": {
			tag:               localTagName,
			localPackage:      "local/provider:latest",
			localController:   &localController,
			wantPackage:       "local/provider:latest",
			wantController:    &localController,
			packageRepository: "registry/provider",
			controllerRepo:    "registry/controller",
		},
		"old registry tag": {
			tag:               "v1.0.0",
			wantPackage:       "registry/provider:v1.0.0",
			wantController:    stringPointer("registry/controller:v1.0.0"),
			packageRepository: "registry/provider",
			controllerRepo:    "registry/controller",
		},
		"new registry tag": {
			tag:               "v1.0.1",
			wantPackage:       "registry/provider:v1.0.1",
			packageRepository: "registry/provider",
			controllerRepo:    "registry/controller",
		},
		"non-semver registry tag": {
			tag:               "latest",
			wantPackage:       "registry/provider:latest",
			packageRepository: "registry/provider",
			controllerRepo:    "registry/controller",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := resolveImageLayout(tt.tag, tt.localPackage, tt.localController, tt.packageRepository, tt.controllerRepo)
			if got.packageImage != tt.wantPackage {
				t.Errorf("package image = %q, want %q", got.packageImage, tt.wantPackage)
			}
			if !reflect.DeepEqual(got.controllerImage, tt.wantController) {
				t.Errorf("controller image = %v, want %v", got.controllerImage, tt.wantController)
			}
		})
	}
}

func TestProviderInstallOptionsControllerImage(t *testing.T) {
	controller := "registry/controller:v1.0.0"

	legacy := providerInstallOptions("provider-cloudfoundry", imageLayout{
		packageImage:    "registry/provider:v1.0.0",
		controllerImage: &controller,
	})
	if legacy.ControllerImage == nil || *legacy.ControllerImage != controller {
		t.Fatalf("legacy controller image = %v, want %q", legacy.ControllerImage, controller)
	}

	single := providerInstallOptions("provider-cloudfoundry", imageLayout{packageImage: "registry/provider:v1.0.1"})
	if single.ControllerImage != nil {
		t.Fatalf("single-image controller image = %q, want nil", *single.ControllerImage)
	}
}

func TestLocalPackageImagesToLoad(t *testing.T) {
	single := imageLayout{packageImage: "local/provider-xpkg:latest"}

	if got := localPackageImagesToLoad(localTagName, "v1.0.0", single, imageLayout{}); !reflect.DeepEqual(got, []string{single.packageImage}) {
		t.Fatalf("local FROM package images = %v, want %v", got, []string{single.packageImage})
	}
	if got := localPackageImagesToLoad("v1.0.0", localTagName, imageLayout{}, single); !reflect.DeepEqual(got, []string{single.packageImage}) {
		t.Fatalf("local TO package images = %v, want %v", got, []string{single.packageImage})
	}
	if got := localPackageImagesToLoad(localTagName, localTagName, single, single); !reflect.DeepEqual(got, []string{single.packageImage}) {
		t.Fatalf("same local package images = %v, want %v", got, []string{single.packageImage})
	}
}

func TestRegistryImagesToLoadCrossLayouts(t *testing.T) {
	legacyController := "registry/controller:v1.0.0"
	legacy := imageLayout{packageImage: "registry/provider:v1.0.0", controllerImage: &legacyController}
	single := imageLayout{packageImage: "registry/provider:v1.0.1"}

	tests := map[string]struct {
		fromTag string
		toTag   string
		from    imageLayout
		to      imageLayout
		want    []string
	}{
		"legacy to single": {
			fromTag: "v1.0.0", toTag: "v1.0.1",
			from: legacy, to: single,
			want: []string{"registry/provider:v1.0.0", "registry/controller:v1.0.0", "registry/provider:v1.0.1"},
		},
		"single to legacy": {
			fromTag: "v1.0.1", toTag: "v1.0.0",
			from: single, to: legacy,
			want: []string{"registry/provider:v1.0.1", "registry/provider:v1.0.0", "registry/controller:v1.0.0"},
		},
		"same tag no duplicate TO load": {
			fromTag: "v1.0.1", toTag: "v1.0.1",
			from: single, to: single,
			want: []string{"registry/provider:v1.0.1"},
		},
		"local to registry": {
			fromTag: localTagName, toTag: "v1.0.1",
			from: imageLayout{packageImage: "local/provider-xpkg:latest"}, to: single,
			want: []string{"registry/provider:v1.0.1"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := registryImagesToLoad(tt.fromTag, tt.toTag, tt.from, tt.to); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("registryImagesToLoad() = %v, want %v", got, tt.want)
			}
		})
	}
}

func stringPointer(value string) *string {
	return &value
}
