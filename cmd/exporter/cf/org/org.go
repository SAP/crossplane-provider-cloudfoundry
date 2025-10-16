package org

import (
	"context"
	"regexp"

	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/guidname"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"

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
	var nameRxs []*regexp.Regexp

	if len(orgNames) > 0 {
		for _, orgName := range orgNames {
			rx, err := regexp.Compile(orgName)
			if err != nil {
				return nil, erratt.Errorf("cannot compile name to regexp: %w", err).With("orgName", orgName)
			}
			nameRxs = append(nameRxs, rx)
		}
	} else {
		nameRxs = []*regexp.Regexp{
			regexp.MustCompile(`.*`),
		}
	}
	orgs, err := cfClient.Organizations.ListAll(ctx, client.NewOrganizationListOptions())
	if err != nil {
		return nil, err
	}

	var results []*resource.Organization
	for _, org := range orgs {
		for _, nameRx := range nameRxs {
			if nameRx.MatchString(org.Name) {
				results = append(results, org)
			}
		}
	}
	return results, nil
}
