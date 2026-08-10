package test

import (
	"strings"
	"testing"
)

func TestParseImagesFromJSON(t *testing.T) {
	controller := "registry.example/controller:v1.2.0"

	tests := map[string]struct {
		input              string
		wantPackage        string
		wantController     *string
		wantErrorSubstring string
	}{
		"single image": {
			input:       `{"crossplane/provider-cloudfoundry":"registry.example/provider:v1.3.0"}`,
			wantPackage: "registry.example/provider:v1.3.0",
		},
		"two images": {
			input:          `{"crossplane/provider-cloudfoundry":"registry.example/provider:v1.2.0","crossplane/provider-cloudfoundry-controller":"registry.example/controller:v1.2.0"}`,
			wantPackage:    "registry.example/provider:v1.2.0",
			wantController: &controller,
		},
		"malformed JSON": {
			input:              `{`,
			wantErrorSubstring: "failed to unmarshal JSON",
		},
		"missing package": {
			input:              `{}`,
			wantErrorSubstring: "non-empty",
		},
		"blank package": {
			input:              `{"crossplane/provider-cloudfoundry":"   "}`,
			wantErrorSubstring: "non-empty",
		},
		"blank supplied controller": {
			input:              `{"crossplane/provider-cloudfoundry":"registry.example/provider:v1.2.0","crossplane/provider-cloudfoundry-controller":" "}`,
			wantErrorSubstring: "empty",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			gotPackage, gotController, err := ParseImagesFromJSON(tt.input)
			if tt.wantErrorSubstring != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrorSubstring) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErrorSubstring, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseImagesFromJSON() error = %v", err)
			}
			if gotPackage != tt.wantPackage {
				t.Errorf("package = %q, want %q", gotPackage, tt.wantPackage)
			}
			if (gotController == nil) != (tt.wantController == nil) {
				t.Fatalf("controller nil = %t, want %t", gotController == nil, tt.wantController == nil)
			}
			if gotController != nil && *gotController != *tt.wantController {
				t.Errorf("controller = %q, want %q", *gotController, *tt.wantController)
			}
		})
	}
}
