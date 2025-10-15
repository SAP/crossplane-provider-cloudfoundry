package space

import (
	"context"

	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/guidname"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

func GetAllNamesFn(ctx context.Context, cfClient *client.Client, orgGuids []string) func() ([]string, error) {
	return func() ([]string, error) {
		resources, err := GetAll(ctx, cfClient, orgGuids, []string{})
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

func GetAll(ctx context.Context, cfClient *client.Client, orgGuids []string, spaceNames []string) ([]*resource.Space, error) {
	listOptions := client.NewSpaceListOptions()
	if len(orgGuids) > 0 {
		listOptions.OrganizationGUIDs.Values = orgGuids
		listOptions.Names.Values = spaceNames
	}
	return cfClient.Spaces.ListAll(ctx, listOptions)
}
