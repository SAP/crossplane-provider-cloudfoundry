package org

import (
	"context"
	"log/slog"
	"regexp"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/cache"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/guidname"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/resources"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/export"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

var (
	c     cache.CacheWithGUIDAndName[*res]
	param = configparam.StringSlice("organization", "Filter for Cloud Foundry organizations").
		WithFlagName("org")
	Org = org{}
)

func init() {
	resources.RegisterKind(Org)
}

type res struct {
	*resource.Organization
}

func (o *res) GetGUID() string {
	return o.GUID
}

func (o *res) GetName() string {
	return o.Name
}

type org struct{}

var _ resources.Kind = org{}

func (o org) Param() configparam.ConfigParam {
	return param
}

func (o org) Export(ctx context.Context, cfClient *client.Client, evHandler export.EventHandler) error {
	orgs, err := o.Get(ctx, cfClient)
	if err != nil {
		return err
	}
	if orgs.Len() == 0 {
		evHandler.Warn(erratt.New("no orgs found", "orgs", param.Value()))
	} else {
		for _, org := range orgs.AllByGUIDs() {
			evHandler.Resource(convertOrgResource(org.Organization))
		}
	}
	return nil
}

func (o org) Get(ctx context.Context, cfClient *client.Client) (cache.CacheWithGUIDAndName[*res], error) {
	if c != nil {
		return c, nil
	}
	param.WithPossibleValuesFn(getAllNamesFn(ctx, cfClient))

	selectedOrgs, err := param.ValueOrAsk(ctx)
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
	c = cache.NewWithGUIDAndName[*res]()
	c.StoreWithGUIDAndName(orgs...)
	slog.Debug("orgs collected", "orgs", c.GetNames())
	return c, nil
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

func getAll(ctx context.Context, cfClient *client.Client, orgNames []string) ([]*res, error) {
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

	var results []*res
	for _, org := range orgs {
		for _, nameRx := range nameRxs {
			if nameRx.MatchString(org.Name) {
				results = append(results, &res{org})
			}
		}
	}
	return results, nil
}
