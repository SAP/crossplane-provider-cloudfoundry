package org

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
)

// GetGUID returns the GUID of an organization by name. It returns an empty string, if the organization does not exist, or there is an error.
func GetGUID(ctx context.Context, c Client, name string) (*string, error) {
	org, err := c.Single(ctx, &client.OrganizationListOptions{Names: client.Filter{Values: []string{name}}})
	if err != nil {
		return nil, err
	}
	return &org.GUID, nil
}
