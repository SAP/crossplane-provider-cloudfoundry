package org

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/pkg/errors"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
)

type OrgScoped interface {
	GetOrgRef() *v1alpha1.OrgReference
}

// / Initialize implements the Initializer interface
// It assumes that orgName is set
func ResolveByName(ctx context.Context, clientFn clients.ClientFn, mg resource.Managed) error {
	cr, ok := mg.(OrgScoped)
	if !ok {
		return errors.New("Cannot resolve org name. The resource does not implement OrgScoped")
	}

	or := cr.GetOrgRef()
	if or == nil {
		return errors.New("Org reference is nil")
	}

	cf, err := clientFn(mg)
	if err != nil {
		return errors.Wrap(err, "Could not connect to Cloud Foundry")
	}
	orgClient := NewClient(cf)
	orgGUID, err := GetGUID(ctx, orgClient, *or.OrgName)
	if err != nil {
		return errors.Wrap(err, "Cannot resolve org reference by name")
	}
	or.Org = orgGUID
	return nil
}

// GetGUID returns the GUID of an organization by name. It returns an empty string, if the organization does not exist, or there is an error.
func GetGUID(ctx context.Context, c Client, name string) (*string, error) {
	org, err := c.Single(ctx, &client.OrganizationListOptions{Names: client.Filter{Values: []string{name}}})
	if err != nil {
		return nil, err
	}
	return &org.GUID, nil
}
