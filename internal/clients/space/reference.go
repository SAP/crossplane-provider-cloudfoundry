package space

import (
	"context"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/pkg/errors"
)

type SpaceScoped interface {
	GetSpaceRef() *v1alpha1.SpaceReference
}

// / Initialize implements the Initializer interface
func ResolveByName(ctx context.Context, clientFn clients.ClientFn, mg resource.Managed) error {
	cr, ok := mg.(SpaceScoped)
	if !ok {
		return errors.New("Cannot resolve space name. The resource does not implement SpaceScoped")
	}

	// if external-name is not set, search by Name and Space
	sr := cr.GetSpaceRef()
	if sr == nil || sr.SpaceName == nil || sr.OrgName == nil {
		if sr.Space != nil { // space GUID is directly set, so we do not need to use names.
			return nil
		}
		return errors.New("Unknown space. Please specify `spaceRef` or `spaceSelector` or using `spaceName` and `orgNames`. ")
	}

	// spaceName and orgName are set, always retrieve space GUID
	cf, err := clientFn(mg)
	if err != nil {
		return errors.Wrap(err, "Could not connect to Cloud Foundry")
	}
	spaceClient, _, orgClient := NewClient(cf)
	spaceGUID := GetGUID(ctx, orgClient, spaceClient, *sr.OrgName, *sr.SpaceName)
	if spaceGUID == "" {
		return errors.Errorf("Cannot find space using spaceName %s and orgName %s", *sr.SpaceName, *sr.OrgName)
	}
	sr.Space = &spaceGUID
	return nil
}
