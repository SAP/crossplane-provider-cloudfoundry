package org

import (
	"context"
	"regexp"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/guidname"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

var (
	cache *Cache
	Param = configparam.StringSlice("org", "Filter for Cloud Foundry organizations").
		WithFlagName("org")
)

func Get(ctx context.Context, cfClient *client.Client) (*Cache, error) {
	if cache != nil {
		return cache, nil
	}
	Param.WithPossibleValuesFn(getAllNamesFn(ctx, cfClient))

	selectedOrgs, err := Param.ValueOrAsk(ctx)
	if err != nil {
		return nil, err
	}

	orgNames := make([]string, len(selectedOrgs))
	for i, orgName := range selectedOrgs {
		name, err := guidname.ParseName(orgName)
		if err != nil {
			return nil, err
		}
		orgNames[i] = name.Name
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	orgs, err := getAll(ctx, cfClient, orgNames)
	if err != nil {
		return nil, err
	}
	cache = newCache(orgs)
	return cache, nil
}

func getAllNamesFn(ctx context.Context, cfClient *client.Client) func() ([]string, error) {
	return func() ([]string, error) {
		resources, err := getAll(ctx, cfClient, []string{})
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

func getAll(ctx context.Context, cfClient *client.Client, orgNames []string) ([]*resource.Organization, error) {
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
