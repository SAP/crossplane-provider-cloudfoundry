package org

import (
	"context"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/uuid"
	"k8s.io/utils/ptr"
)

// Client is the interface that defines the methods that a Org client should implement.
type Client interface {
	Get(context.Context, string) (*resource.Organization, error)
	Single(context.Context, *client.OrganizationListOptions) (*resource.Organization, error)
	Create(context.Context, *resource.OrganizationCreate) (*resource.Organization, error)
}

// Resource is the type that implements the resource.Resource interface for a Org.
type Resource resource.Organization

// NewClient creates a new client instance from a cfclient.ServiceInstance instance.
func NewClient(cf *client.Client) Client {
	return cf.Organizations
}

// GetByIDOrName returns an organization by ID or Name.
func GetByIDOrName(ctx context.Context, c Client, id, name string) (*resource.Organization, error) {

	_, err := uuid.Parse(id)
	if err == nil {
		return c.Get(ctx, id)
	}

	return c.Single(ctx, &client.OrganizationListOptions{Names: client.Filter{Values: []string{name}}})
}

// GenerateCreate generates the OrganizationCreate from an *OrgParameters
func GenerateCreate(spec v1alpha1.OrgParameters) *resource.OrganizationCreate {
	// if external-name is not set, search by Name and Space
	create := &resource.OrganizationCreate{}
	create.Name = spec.Name
	create.Suspended = spec.Suspended

	// TODO: ADD labels and annotations

	return create
}

// GenerateObservation takes an Organization resource and returns *OrgObservation.
func GenerateObservation(o *resource.Organization) v1alpha1.OrgObservation {
	obs := v1alpha1.OrgObservation{
		ID:        ptr.To(o.GUID),
		CreatedAt: ptr.To(o.CreatedAt.Format(time.RFC3339)),
		UpdatedAt: ptr.To(o.UpdatedAt.Format(time.RFC3339)),
		Suspended: o.Suspended,
	}

	if o.Metadata != nil {
		obs.Annotations = o.Metadata.Annotations
		obs.Labels = o.Metadata.Labels
	}

	if o.Relationships.Quota.Data != nil {
		obs.Quota = ptr.To(o.Relationships.Quota.Data.GUID)
	}
	return obs
}

// LateInitialize fills the unassigned fields with values from a Organization resource.
func LateInitialize(spec *v1alpha1.OrgParameters, from *resource.Organization) {

	if spec.Name == "" {
		spec.Name = from.Name
	}

	if spec.Suspended == nil {
		spec.Suspended = from.Suspended
	}
	// TODO: ADD labels and annotations
}

// IsUpToDate checks whether current state is up-to-date compared to the given
// set of parameters.
func IsUpToDate(spec v1alpha1.OrgParameters, observed *resource.Organization) bool {
	// return always true, as for now the Org resource is observe only

	return spec.Name == observed.Name
}
