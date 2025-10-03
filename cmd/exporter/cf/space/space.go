package space

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

func GetAll(ctx context.Context, cfClient *client.Client, orgGuids []string) ([]*resource.Space, error) {
	listOptions := client.NewSpaceListOptions()
	if len(orgGuids) > 0 {
		listOptions.OrganizationGUIDs.Values = orgGuids
	}
	return cfClient.Spaces.ListAll(ctx, listOptions)
}
