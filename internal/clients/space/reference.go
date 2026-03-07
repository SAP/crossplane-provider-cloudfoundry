package space

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/pkg/errors"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/org"
)

type SpaceScoped interface {
	GetSpaceRef() *v1alpha1.SpaceReference
}

// ResolveByName resolves the space reference by name.
func ResolveByName(ctx context.Context, clientFn clients.ClientFn, mg resource.Managed) error {
	cr, ok := mg.(SpaceScoped)
	if !ok {
		return errors.New("Cannot resolve space name. The resource does not implement SpaceScoped")
	}

	sr := cr.GetSpaceRef()
	// resolve space by name only if spaceName and orgName are set
	if sr != nil && sr.SpaceName != nil && sr.OrgName != nil {
		cf, err := clientFn(mg)
		if err != nil {
			return errors.Wrap(err, "Could not connect to Cloud Foundry")
		}
		spaceClient, _, orgClient := NewClient(cf)
		spaceGUID, err := GetGUID(ctx, orgClient, spaceClient, *sr.OrgName, *sr.SpaceName)
		if err != nil {
			return errors.Wrap(err, "Cannot resolve space reference by name")
		}
		sr.Space = spaceGUID
		return nil
	}

	// nothing to resolve.
	return nil
}

// GetGUID returns the GUID of a space by name. It returns an empty string, if the space does not exist, or there is an error.
func GetGUID(ctx context.Context, orgClient org.Client, spaceClient Space, orgName, spaceName string) (*string, error) {
	if spaceName == "" {
		return nil, errors.New("spaceName is empty")
	}
	opts := client.NewSpaceListOptions()
	opts.Names = client.Filter{Values: []string{spaceName}}

	if orgName != "" { // optionally filter by orgName
		orgGUID, err := org.GetGUID(ctx, orgClient, orgName)
		if err != nil {
			return nil, errors.Wrap(err, "Cannot resolve org reference by name")
		}
		opts.OrganizationGUIDs = client.Filter{Values: []string{*orgGUID}}
	}

	space, err := spaceClient.Single(ctx, opts)
	if err != nil {
		return nil, err
	}

	return &space.GUID, nil
}
