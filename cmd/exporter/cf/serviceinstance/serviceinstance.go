package serviceinstance

import (
	"context"

	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/guidname"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

func GetAllNamesFn(ctx context.Context, cfClient *client.Client, orgGuids, spaceGuids []string) func() ([]string, error) {
	return func() ([]string, error) {
		resources, err := GetAll(ctx, cfClient, orgGuids, spaceGuids, []string{})
		if err != nil {
			return nil, err
		}
		names := make([]string, len(resources))
		for i, res := range resources {
			names[i] = guidname.NewName(res.GUID, res.Name).String()
		}
		return names, nil
	}
}

func GetAll(ctx context.Context, cfClient *client.Client, orgGuids, spaceGuids, serviceInstanceNames []string) ([]*resource.ServiceInstance, error) {
	listOptions := client.NewServiceInstanceListOptions()
	listOptions.OrganizationGUIDs.Values = orgGuids
	listOptions.SpaceGUIDs.Values = spaceGuids
	listOptions.Names.Values = serviceInstanceNames
	return cfClient.ServiceInstances.ListAll(ctx, listOptions)
}
