package space

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/guidname"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/org"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/resources"

	"github.com/SAP/xp-clifford/cli/configparam"
	"github.com/SAP/xp-clifford/cli/export"
	"github.com/SAP/xp-clifford/erratt"
	"github.com/SAP/xp-clifford/mkcontainer"
	"github.com/SAP/xp-clifford/parsan"
	"github.com/SAP/xp-clifford/yaml"
	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	v1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
)

var (
	c     mkcontainer.Container
	param = configparam.StringSlice("space", "Filter for Cloud Foundry spaces").
		WithFlagName("space")
)

func init() {
	resources.RegisterKind(space{})
}

type res struct {
	*resource.Space
	*yaml.ResourceWithComment
}

func (r *res) GetGUID() string {
	return r.GUID
}

func (r *res) GetName() string {
	names := parsan.ParseAndSanitize(r.Name, parsan.RFC1035LowerSubdomain)
	if len(names) == 0 {
		r.AddComment(fmt.Sprintf("error sanitizing name: %s", r.Name))
		return r.Name
	}
	return names[0]
}

type space struct{}

var _ resources.Kind = space{}

func (s space) Param() configparam.ConfigParam {
	return param
}
func (s space) KindName() string {
	return param.GetName()
}

func (s space) Export(ctx context.Context, cfClient *client.Client, evHandler export.EventHandler, resolveReferences bool) error {
	spaces, err := Get(ctx, cfClient)
	if err != nil {
		return err
	}
	if spaces.IsEmpty() {
		evHandler.Warn(erratt.New("no space found", "spaces", param.Value()))
	} else {
		for _, space := range spaces.AllByGUIDs() {
			evHandler.Resource(convertSpaceResource(ctx, cfClient, space.(*res), evHandler, resolveReferences))
		}
	}
	return nil
}

func Get(ctx context.Context, cfClient *client.Client) (mkcontainer.Container, error) {
	if c != nil {
		return c, nil
	}
	orgs, err := org.Get(ctx, cfClient)
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
	c = mkcontainer.New()
	c.Store(spaces...)
	slog.Debug("spaces collected", "spaces", c.GetNames())
	return c, nil
}

func ResolveReference(ctx context.Context, cfClient *client.Client, spaceRef *v1alpha1.SpaceReference) error {
	if spaceRef.Space == nil {
		panic("spaceRef.Space not set")
	}
	spaces, err := Get(ctx, cfClient)
	if err != nil {
		return erratt.Errorf("cannot get orgs: %w", err)
	}
	space := spaces.GetByGUID(*spaceRef.Space)
	if space == nil {
		return erratt.New("space reference not found", "GUID", *spaceRef.Space)
	}
	spaceRef.SpaceRef = &v1.NamespacedReference{
		Name: space.(mkcontainer.ItemWithName).GetName(),
	}
	spaceRef.Space = nil
	return nil
}

func getAllNamesFn(ctx context.Context, cfClient *client.Client, orgGuids []string) func() ([]string, error) {
	return func() ([]string, error) {
		resources, err := getAll(ctx, cfClient, orgGuids, []string{})
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

func getAll(ctx context.Context, cfClient *client.Client, orgGuids []string, spaceNames []string) ([]mkcontainer.Item, error) {
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

	var results []mkcontainer.Item
	for _, space := range spaces {
		for _, nameRx := range nameRxs {
			if nameRx.MatchString(space.Name) {
				results = append(results, &res{
					ResourceWithComment: yaml.NewResourceWithComment(nil),
					Space:               space,
				})
			}
		}
	}
	return results, nil
}
