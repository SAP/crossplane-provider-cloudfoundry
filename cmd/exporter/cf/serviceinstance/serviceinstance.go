package serviceinstance

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

func GetAll(ctx context.Context, cfClient *client.Client, orgGuids, spaceGuids []string) ([]*resource.ServiceInstance, error) {
	listOptions := client.NewServiceInstanceListOptions()
	if len(orgGuids) > 0 {
		listOptions.OrganizationGUIDs.Values = orgGuids
	}
	if len(spaceGuids) > 0 {
		listOptions.SpaceGUIDs.Values = spaceGuids
	}
	return cfClient.ServiceInstances.ListAll(ctx, listOptions)
}
