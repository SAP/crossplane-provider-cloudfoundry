package config

import (
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/client"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/resource"
)

// ProviderConfig defines the configuration for a provider
type ProviderConfig interface {
	// GetProviderConfigRef returns the provider config reference
	GetProviderConfigRef() client.ProviderConfigRef

	// Validate validates the configuration
	Validate() bool
}

// ConfigParser parses configuration for resources to import
type ConfigParser interface {
	// ParseConfig parses the configuration file
	ParseConfig(configPath string) (ProviderConfig, []resource.ResourceFilter, error)
}
