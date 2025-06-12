package v1alpha1

import (
	"context"
	"fmt"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/adapters"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/client"
)

// BaseAdapter provides common functionality for all adapters
type BaseAdapter struct {
	adapters.CFClient
}

// Connect establishes a connection to the provider
func (a *BaseAdapter) Connect(ctx context.Context, creds client.Credentials) error {
	cfCreds, ok := creds.(*adapters.CFCredentials)
	if !ok {
		return fmt.Errorf("invalid credentials type")
	}

	// Set the client using the adapter
	adapter := &adapters.CFClientAdapter{}
	providerClient, err := adapter.BuildClient(ctx, cfCreds)
	if err != nil {
		return fmt.Errorf("failed to build client: %w", err)
	}

	// Type assert to get the CFClient
	cfClient, ok := providerClient.(*adapters.CFClient)
	if !ok {
		return fmt.Errorf("failed to get CF client")
	}

	// Set the client in the adapter
	a.CFClient = *cfClient
	return nil
}
