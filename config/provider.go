/*
Copyright 2023 SAP SE
*/

package config

import (
	// Note(turkenh): we are importing this to embed provider schema document
	_ "embed"

	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/config/app"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/config/domain"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/config/route"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/config/space"

	"github.com/crossplane/upjet/pkg/config"
)

const (
	resourcePrefix = "cloudfoundry"
	modulePath     = "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry"
)

//go:embed schema.json
var providerSchema string

//go:embed provider-metadata.yaml
var providerMetadata string

// GetProvider returns provider configuration
func GetProvider() *config.Provider {
	pc := config.NewProvider([]byte(providerSchema), resourcePrefix, modulePath, []byte(providerMetadata),
		config.WithRootGroup("btp.orchestrate.cloud.sap"),
		config.WithIncludeList(ExternalNameConfigured()),
		config.WithDefaultResourceOptions(
			ExternalNameConfigurations(),
			CustomReferenceConfigurations(),
		))

	for _, configure := range []func(provider *config.Provider){
		// add custom config functions
		app.Configure,
		route.Configure,
		domain.Configure,
		space.Configure,
	} {
		configure(pc)
	}

	pc.ConfigureResources()
	return pc
}
