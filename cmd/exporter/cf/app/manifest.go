// Package app implements Cloud Foundry App resource export functionality.
package app

import (
	"context"
	"log/slog"

	// kyaml "sigs.k8s.io/yaml"
	gyaml "gopkg.in/yaml.v2"

	"github.com/SAP/xp-clifford/erratt"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/operation"
)

// type docker struct {
// 	Image string `json:"image"`
// 	Username *string `json:"username,omitempty"`
// }

// type application struct {
// 	Name string `json:"name"`
// 	Docker *docker `json:"docker,omitempty"`
// }

// type manifest struct {
// 	Applications []application `json:"applications"`
// }

// getManifest fetches the application manifest from Cloud Foundry API for the given app GUID.
// Returns the parsed manifest containing all application configurations.
func getManifest(ctx context.Context, cfClient *client.Client, appGUID string) (*operation.Manifest, error) {
	m := &operation.Manifest{}
	stringManifest, err := cfClient.Manifests.Generate(ctx, appGUID)
	if err != nil {
		return nil, erratt.Errorf("cannot generate app manifest: %w", err).With("GUID", appGUID)
	}

	slog.Debug("manifest fetched", "manifest", stringManifest)
	err = gyaml.Unmarshal([]byte(stringManifest), m)

	return m, err
}

// getAppManifest retrieves the first application manifest from the CF API response.
// Returns nil if no applications are found in the manifest.
func getAppManifest(ctx context.Context, cfClient *client.Client, appGUID string) (*operation.AppManifest, error) {
	m, err := getManifest(ctx, cfClient, appGUID)
	if err != nil {
		return nil, err
	}
	if len(m.Applications) == 0 {
		return nil, nil
	}
	if m.Applications[0] == nil {
		return nil, nil
	}
	return m.Applications[0], nil
}
