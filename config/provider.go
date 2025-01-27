/*
Copyright 2023 SAP SE
*/

package config

import (
	// Note(turkenh): we are importing this to embed provider schema document
	_ "embed"

	"github.com/crossplane/upjet/pkg/config"

	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/config/app"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/config/domain"
)

const (
	resourcePrefix = "cloudfoundry"              // resourcePrefix is the prefix of terraform resources
	apiRootGroup   = "btp.orchestrate.cloud.sap" // apiRootGroup is the root group of the API
	apiVersion     = "v1alpha1"
	modulePath     = "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry"
)

//go:embed schema.json
var providerSchema string

//go:embed provider-metadata.yaml
var providerMetadata string

// GetProvider returns provider configuration
func GetProvider() *config.Provider {
	pc := config.NewProvider([]byte(providerSchema), resourcePrefix, modulePath, []byte(providerMetadata),
		config.WithRootGroup(apiRootGroup),
		config.WithIncludeList(ExternalNameConfigured()),
		config.WithFeaturesPackage("internal/features"),
		config.WithDefaultResourceOptions(
			ExternalNameConfigurations(),
			CustomReferenceConfigurations(),
			apiVersionConfigurations(apiVersion),
		))

	for _, configure := range []func(provider *config.Provider){
		// add custom config functions
		app.Configure,
		domain.Configure,
		// space.Configure,
	} {
		configure(pc)
	}

	pc.ConfigureResources()
	return pc
}

// apiVersionConfigurations configures API version for all upjet resources
func apiVersionConfigurations(version string) config.ResourceOption {
	return func(r *config.Resource) {
		r.Version = version
	}
}
