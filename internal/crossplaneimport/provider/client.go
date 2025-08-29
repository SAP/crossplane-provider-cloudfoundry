package provider

import (
	"context"
)

// ProviderClient represents a client for interacting with a provider
type ProviderClient interface {
	// GetResourcesByType fetches resources of a specific type
	GetResourcesByType(ctx context.Context, resourceType string, filter map[string]string) ([]interface{}, error)
}

// Credentials represents authentication credentials for a provider
type Credentials interface{}

// ClientAdapter adapts provider-specific client creation
type ClientAdapter interface {
	// BuildClient builds a client for the provider
	BuildClient(ctx context.Context, credentials Credentials) (ProviderClient, error)
}
