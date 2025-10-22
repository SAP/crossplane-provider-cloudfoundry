package space

import (
	"context"
	"regexp"

	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/guidname"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"

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
	var nameRxs []*regexp.Regexp

	if len(spaceNames) > 0 {
		for _, spaceName := range spaceNames {
			rx, err := regexp.Compile(spaceName)
			if err != nil {
				return nil, erratt.Errorf("cannot compile name to regexp: %w", err).With("spaceName", spaceName)
			}
			nameRxs = append(nameRxs, rx)
		}
	} else {
		nameRxs = []*regexp.Regexp{
			regexp.MustCompile(`.*`),
		}
	}

	listOptions := client.NewSpaceListOptions()
	if len(orgGuids) > 0 {
		listOptions.OrganizationGUIDs.Values = orgGuids
	}
	spaces, err := cfClient.Spaces.ListAll(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	var results []*resource.Space
	for _, space := range spaces {
		for _, nameRx := range nameRxs {
			if nameRx.MatchString(space.Name) {
				results = append(results, space)
			}
		}
	}
	return results, nil
}
