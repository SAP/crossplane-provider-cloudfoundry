package app

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/guidname"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/org"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/resources"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/space"

	"github.com/SAP/xp-clifford/cli/configparam"
	"github.com/SAP/xp-clifford/cli/export"
	"github.com/SAP/xp-clifford/erratt"
	"github.com/SAP/xp-clifford/mkcontainer"
	"github.com/SAP/xp-clifford/parsan"
	"github.com/SAP/xp-clifford/yaml"
	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

var (
	c     mkcontainer.TypedContainer[*res]
	param = configparam.StringSlice("app", "Filter for Cloud Foundry apps").
		WithFlagName("app")
)

func init() {
	resources.RegisterKind(app{})
}

type res struct {
	*resource.App
	*yaml.ResourceWithComment
}

func (r *res) GetGUID() string {
	return r.GUID
}

func (r *res) GetName() string {
	name := r.Name
	names := parsan.ParseAndSanitize(name, parsan.RFC1035LowerSubdomain)
	if len(names) == 0 {
		r.AddComment(fmt.Sprintf("error sanitizing name: %s", name))
	} else {
		name = names[0]
	}
	return name
}

type app struct{}

var _ resources.Kind = app{}

func (a app) Param() configparam.ConfigParam {
	return param
}

func (a app) KindName() string {
	return param.GetName()
}

func (a app) Export(ctx context.Context, cfClient *client.Client, evHandler export.EventHandler, resolveReferences bool) error {
	apps, err := Get(ctx, cfClient)
	if err != nil {
		return err
	}
	if apps.IsEmpty() {
		evHandler.Warn(erratt.New("no apps found", "apps", param.Value()))
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	for _, app := range apps.AllByGUIDs() {
		slog.Debug("exporting app", "name", app.Name)
		evHandler.Resource(convertAppResource(ctx, cfClient, app, evHandler, resolveReferences))
	}

	return nil
}

func getAllNamesFn(ctx context.Context, cfClient *client.Client, orgGuids, spaceGuids []string) func() ([]string, error) {
	return func() ([]string, error) {
		resources, err := getAll(ctx, cfClient, orgGuids, spaceGuids, []string{})
		if err != nil {
			return nil, err
		}
		names := make([]string, len(resources))
		for i, res := range resources {
			names[i] = guidname.NewName(res).String()
		}
		return names, nil
	}
}

func Get(ctx context.Context, cfClient *client.Client) (mkcontainer.TypedContainer[*res], error) {
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

	selectedApps, err := param.ValueOrAsk(ctx)
	if err != nil {
		return nil, err
	}

	appNames := make([]string, len(selectedApps))
	for i, appName := range selectedApps {
		name, err := guidname.ParseName(appName)
		if err != nil {
			return nil, err
		}
		appNames[i] = name.Name
	}
	slog.Debug("apps selected", "apps", selectedApps, "appNames", appNames)
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	apps, err := getAll(ctx,
		cfClient,
		orgs.GetGUIDs(),
		spaces.GetGUIDs(),
		appNames,
	)
	if err != nil {
		return nil, err
	}
	c = mkcontainer.NewTyped[*res]()
	c.Store(apps...)
	slog.Debug("apps collected", "apps", c.GetNames())
	return c, nil
}

func getAll(ctx context.Context, cfClient *client.Client, orgGuids, spaceGuids, appNames []string) ([]*res, error) {
	var nameRxs []*regexp.Regexp

	if len(appNames) > 0 {
		for _, appName := range appNames {
			slog.Debug("processing app", "name", appName)
			rx, err := regexp.Compile(appName)
			if err != nil {
				return nil, erratt.Errorf("cannot compile name to regexp: %w", err).With("appName", appName)
			}
			nameRxs = append(nameRxs, rx)
		}
	} else {
		nameRxs = []*regexp.Regexp{
			regexp.MustCompile(`.*`),
		}
	}

	listOptions := client.NewAppListOptions()
	if len(orgGuids) > 0 {
		listOptions.OrganizationGUIDs.Values = orgGuids
		listOptions.SpaceGUIDs.Values = spaceGuids
	}
	apps, err := cfClient.Applications.ListAll(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	var results []*res
	for _, app := range apps {
		for _, nameRx := range nameRxs {
			if nameRx.MatchString(app.Name) {
				slog.Debug("matching app found", "rx", nameRx.String(), "found", app.Name)
				results = append(results, &res{
					ResourceWithComment: yaml.NewResourceWithComment(nil),
					App:                 app,
				})
			}
		}
	}
	return results, nil
}
