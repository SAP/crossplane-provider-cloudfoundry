package org

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

func GetAll(ctx context.Context, cfClient *client.Client) ([]*resource.Organization, error) {
	return cfClient.Organizations.ListAll(ctx, client.NewOrganizationListOptions())
}
