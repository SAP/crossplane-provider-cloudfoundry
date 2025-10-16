package serviceinstance

import (
	"context"
	"regexp"

	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/guidname"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"

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
