package org

import (
	"context"

	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/guidname"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

func GetAllNamesFn(ctx context.Context, cfClient *client.Client) func() ([]string, error) {
	return func() ([]string, error) {
		resources, err := GetAll(ctx, cfClient, []string{})
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

func GetAll(ctx context.Context, cfClient *client.Client, orgNames []string) ([]*resource.Organization, error) {
	listOptions := client.NewOrganizationListOptions()
	listOptions.Names.Values = orgNames
	return cfClient.Organizations.ListAll(ctx, listOptions)
}
