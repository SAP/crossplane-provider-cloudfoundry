package serviceinstance

import (
	"context"
	"regexp"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/guidname"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/org"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/space"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

var (
	cache *Cache
	Param = configparam.StringSlice("serviceinstance", "Filter for Cloud Foundry service instances").
		WithFlagName("serviceinstance")
)

func Get(ctx context.Context, cfClient *client.Client) (*Cache, error) {
	if cache != nil {
		return cache, nil
	}

	orgs, err := org.Get(ctx, cfClient)
	if err != nil {
		return nil, err
	}

	spaces, err := space.Get(ctx, cfClient)
	if err != nil {
		return nil, err
	}

	Param.WithPossibleValuesFn(getAllNamesFn(ctx, cfClient, orgs.GetGUIDs(), spaces.GetGUIDs()))

	selectedServiceInstances, err := Param.ValueOrAsk(ctx)
	if err != nil {
		return nil, err
	}

	serviceInstanceNames := make([]string, len(selectedServiceInstances))
	for i, serviceInstanceName := range selectedServiceInstances {
		name, err := guidname.ParseName(serviceInstanceName)
		if err != nil {
			return nil, err
		}
		serviceInstanceNames[i] = name.Name
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	serviceInstances, err := getAll(ctx,
		cfClient,
		orgs.GetGUIDs(),
		spaces.GetGUIDs(),
		serviceInstanceNames,
	)
	if err != nil {
		return nil, err
	}
	cache = newCache(serviceInstances)
	return cache, nil
}

func getAllNamesFn(ctx context.Context, cfClient *client.Client, orgGuids, spaceGuids []string) func() ([]string, error) {
	return func() ([]string, error) {
		resources, err := getAll(ctx, cfClient, orgGuids, spaceGuids, []string{})
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

func getAll(ctx context.Context, cfClient *client.Client, orgGuids, spaceGuids, serviceInstanceNames []string) ([]*resource.ServiceInstance, error) {
	var nameRxs []*regexp.Regexp

	if len(serviceInstanceNames) > 0 {
		for _, serviceInstanceName := range serviceInstanceNames {
			rx, err := regexp.Compile(serviceInstanceName)
			if err != nil {
				return nil, erratt.Errorf("cannot compile name to regexp: %w", err).With("serviceInstanceName", serviceInstanceName)
			}
			nameRxs = append(nameRxs, rx)
		}
	} else {
		nameRxs = []*regexp.Regexp{
			regexp.MustCompile(`.*`),
		}
	}

	listOptions := client.NewServiceInstanceListOptions()
	if len(orgGuids) > 0 {
		listOptions.OrganizationGUIDs.Values = orgGuids
		listOptions.SpaceGUIDs.Values = spaceGuids
	}
	serviceInstances, err := cfClient.ServiceInstances.ListAll(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	var results []*resource.ServiceInstance
	for _, serviceInstance := range serviceInstances {
		for _, nameRx := range nameRxs {
			if nameRx.MatchString(serviceInstance.Name) {
				results = append(results, serviceInstance)
			}
		}
	}
	return results, nil
}
