package serviceinstance

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/cache"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/guidname"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/org"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/resources"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/space"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/erratt"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/parsan"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/yaml"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

var (
	c     cache.CacheWithGUIDAndName[*res]
	param = configparam.StringSlice("serviceinstance", "Filter for Cloud Foundry service instances").
		WithFlagName("serviceinstance")
)

func init() {
	resources.RegisterKind(serviceinstance{})
}

type res struct {
	*resource.ServiceInstance
	*yaml.ResourceWithComment
}

func (r *res) GetGUID() string {
	return r.GUID
}

func (r *res) GetName() string {
	name := r.Name
	names := parsan.ParseAndSanitize(
		name,
		parsan.RFC1035Subdomain,
	)
	if len(names) == 0 {
		r.AddComment(fmt.Sprintf("error sanitizing name: %s", name))
	} else {
		name = names[0]
	}
	return name
}

type serviceinstance struct{}

var _ resources.Kind = serviceinstance{}

func (si serviceinstance) Param() configparam.ConfigParam {
	return param
}

func (si serviceinstance) KindName() string {
	return param.GetName()
}

func (si serviceinstance) Export(ctx context.Context, cfClient *client.Client, evHandler export.EventHandler, resolveReferences bool) error {
	serviceInstances, err := Get(ctx, cfClient)
	if err != nil {
		return err
	}
	if serviceInstances.Len() == 0 {
		evHandler.Warn(erratt.New("no serviceinstance found", "serviceinstances", param.Value()))
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	for _, sInstance := range serviceInstances.AllByGUIDs() {
		slog.Debug("exporting serviceinstance", "name", sInstance.ServiceInstance.Name)
		evHandler.Resource(convertServiceInstanceResource(ctx, cfClient, sInstance, evHandler, resolveReferences))
	}
	return nil
}

func Get(ctx context.Context, cfClient *client.Client) (cache.CacheWithGUIDAndName[*res], error) {
	if c != nil {
		return c, nil
	}

	orgs, err := org.Get(ctx, cfClient)
	if err != nil {
		return nil, err
	}

	spaces, err := space.Get(ctx, cfClient)
	if err != nil {
		return nil, err
	}

	param.WithPossibleValuesFn(getAllNamesFn(ctx, cfClient, orgs.GetGUIDs(), spaces.GetGUIDs()))

	selectedServiceInstances, err := param.ValueOrAsk(ctx)
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
	c = cache.NewWithGUIDAndName[*res]()
	c.StoreWithGUIDAndName(serviceInstances...)
	slog.Debug("serviceinstances collected", "serviceinstances", c.GetNames())
	return c, nil
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

func getAll(ctx context.Context, cfClient *client.Client, orgGuids, spaceGuids, serviceInstanceNames []string) ([]*res, error) {
	var nameRxs []*regexp.Regexp

	if len(serviceInstanceNames) > 0 {
		for _, serviceInstanceName := range serviceInstanceNames {
			slog.Debug("processing serviceInstance", "name", serviceInstanceName)
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

	var results []*res
	for _, serviceInstance := range serviceInstances {
		for _, nameRx := range nameRxs {
			if nameRx.MatchString(serviceInstance.Name) {
				slog.Debug("matching serviceInstance found", "rx", nameRx.String(), "found", serviceInstance.Name)
				results = append(results, &res{
					ResourceWithComment: yaml.NewResourceWithComment(nil),
					ServiceInstance:     serviceInstance,
				})
			}
		}
	}
	return results, nil
}
