package domain

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/pkg/errors"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
)

type DomainScoped interface {
	GetDomainRef() *v1alpha1.DomainReference
}

// / Initialize implements the Initializer interface
func ResolveByName(ctx context.Context, clientFn clients.ClientFn, mg resource.Managed) error {
	cr, ok := mg.(DomainScoped)
	if !ok {
		return errors.New("Cannot resolve domain name. The resource does not implement DomainScoped")
	}

	// if external-name is not set, search by Name and Domain
	dr := cr.GetDomainRef()
	if dr == nil || dr.DomainName == nil {
		if dr.Domain != nil { // domain GUID is directly set, so we do not need to use names.
			return nil
		}
		return errors.New("Unknown domain. Please specify `domainRef` or `domainSelector` or using `domainName`. ")
	}

	// domainName is set, always retrieve domain GUID
	cf, err := clientFn(mg)
	if err != nil {
		return errors.Wrap(err, "Could not connect to Cloud Foundry")
	}
	domainClient := NewClient(cf)
	domainGUID, err := GetGUID(ctx, domainClient, *dr.DomainName)
	if err != nil {
		return errors.Wrap(err, "Cannot resolve space reference by name")
	}
	dr.Domain = domainGUID
	return nil
}

// GetGUID returns the GUID of a space by name. It returns an empty string, if the space does not exist, or there is an error.
func GetGUID(ctx context.Context, domainClient Client, domainName string) (*string, error) {
	if domainName == "" {
		return nil, errors.New("domainName is empty")
	}
	opts := client.NewDomainListOptions()
	opts.Names = client.Filter{Values: []string{domainName}}

	domain, err := domainClient.Single(ctx, opts)
	if err != nil {
		return nil, err
	}

	return &domain.GUID, nil
}
