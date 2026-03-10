package org

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/guidname"
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
	param = configparam.StringSlice("organization", "Filter for Cloud Foundry organizations").
		WithFlagName("org")
)

func init() {
	resources.RegisterKind(org{})
}

type res struct {
	*resource.Organization
	*yaml.ResourceWithComment
}

var (
	_ mkcontainer.ItemWithGUID = &res{}
	_ mkcontainer.ItemWithName = &res{}
)

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

type org struct{}

var _ resources.Kind = org{}

func (o org) Param() configparam.ConfigParam {
	return param
}

func (o org) KindName() string {
	return param.GetName()
}

func (o org) Export(ctx context.Context, cfClient *client.Client, evHandler export.EventHandler, resolveReferences bool) error {
	orgs, err := Get(ctx, cfClient)
	if err != nil {
		return err
	}
	if orgs.IsEmpty() {
		evHandler.Warn(erratt.New("no orgs found", "orgs", param.Value()))
	} else {
		for _, org := range orgs.AllByGUIDs() {
			evHandler.Resource(convertOrgResource(org.(*res)))
		}
	}
	return nil
}

func Get(ctx context.Context, cfClient *client.Client) (mkcontainer.Container, error) {
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
	c = mkcontainer.New()
	c.Store(orgs...)
	slog.Debug("orgs collected", "orgs", c.GetNames())
	return c, nil
}

func ResolveReference(ctx context.Context, cfClient *client.Client, orgRef *v1alpha1.OrgReference) error {
	if orgRef.Org == nil {
		panic("orgRef.Org not set")
	}
	orgs, err := Get(ctx, cfClient)
	if err != nil {
		return erratt.Errorf("cannot get orgs: %w", err)
	}
	org := orgs.GetByGUID(*orgRef.Org)
	if org == nil {
		return erratt.New("space reference not found", "GUID", *orgRef.Org)
	}
	orgRef.OrgRef = &v1.NamespacedReference{
		Name: org.(mkcontainer.ItemWithName).GetName(),
	}
	orgRef.Org = nil
	return nil
}

func getAllNamesFn(ctx context.Context, cfClient *client.Client) func() ([]string, error) {
	return func() ([]string, error) {
		resources, err := getAll(ctx, cfClient, []string{})
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

func getAll(ctx context.Context, cfClient *client.Client, orgNames []string) ([]mkcontainer.Item, error) {
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

	var results []mkcontainer.Item
	for _, org := range orgs {
		for _, nameRx := range nameRxs {
			if nameRx.MatchString(org.Name) {
				results = append(results, &res{
					ResourceWithComment: yaml.NewResourceWithComment(nil),
					Organization:        org,
				})
			}
		}
	}
	return results, nil
}
