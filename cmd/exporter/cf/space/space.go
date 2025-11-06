package space

import (
	"context"
	"log/slog"
	"regexp"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/guidname"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/org"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/resources"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/export"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

func init() {
	resources.RegisterKind(param.Name, Space)
}

type space struct{}

var _ resources.Kind = space{}

var (
	cache *Cache
	param = configparam.StringSlice("space", "Filter for Cloud Foundry spaces").
		WithFlagName("space")
	Space = space{}
)

func (s space) Param() configparam.ConfigParam {
	return param
}

func (s space) Export(ctx context.Context, cfClient *client.Client, evHandler export.EventHandler) error {
	spaces, err := s.Get(ctx, cfClient)
	if err != nil {
		return err
	}
	if spaces.Len() == 0 {
		evHandler.Warn(erratt.New("no space found", "spaces", param.Value()))
	} else {
		spaces.Export(evHandler)
	}
	return nil
}

func (s space) Get(ctx context.Context, cfClient *client.Client) (*Cache, error) {
	if cache != nil {
		return cache, nil
	}
	orgs, err := org.Org.Get(ctx, cfClient)
	if err != nil {
		return nil, err
	}
	param.WithPossibleValuesFn(getAllNamesFn(ctx, cfClient, orgs.GetGUIDs()))

	selectedSpaces, err := param.ValueOrAsk(ctx)
	if err != nil {
		return nil, err
	}

	spaceNames := make([]string, len(selectedSpaces))
	for i, spaceName := range selectedSpaces {
		name, err := guidname.ParseName(spaceName)
		if err != nil {
			return nil, err
		}
		spaceNames[i] = name.Name
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	spaces, err := getAll(ctx, cfClient, orgs.GetGUIDs(), spaceNames)
	if err != nil {
		return nil, erratt.Errorf("cannot get spaces: %w", err)
	}
	cache = newCache(spaces)
	param.WithPossibleValuesFn(convertPossibleValuesFn(cache.GetNames))
	slog.Debug("spaces collected", "spaces", cache.GetNames())
	return cache, nil
}

func convertPossibleValuesFn(fn func() []string) func() ([]string, error) {
	return func() ([]string, error) {
		return fn(), nil
	}
}

func getAllNamesFn(ctx context.Context, cfClient *client.Client, orgGuids []string) func() ([]string, error) {
	return func() ([]string, error) {
		resources, err := getAll(ctx, cfClient, orgGuids, []string{})
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

func getAll(ctx context.Context, cfClient *client.Client, orgGuids []string, spaceNames []string) ([]*resource.Space, error) {
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
