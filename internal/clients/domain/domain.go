package domain

import (
	"context"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/uuid"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

// Client is the interface that defines the methods that a Domain client should implement.
type Client interface {
	Get(context.Context, string) (*resource.Domain, error)
	Single(context.Context, *client.DomainListOptions) (*resource.Domain, error)
	Create(context.Context, *resource.DomainCreate) (*resource.Domain, error)
	Delete(context.Context, string) (string, error)
	Update(context.Context, string, *resource.DomainUpdate) (*resource.Domain, error)
}

// Resource is the type that implements the resource.Resource interface for a Domain.
type Resource resource.Domain

// NewClient creates a new client instance from a cfclient.Domain instance.
func NewClient(cf *client.Client) Client {
	return cf.Domains
}

// GetByIDOrName returns a domain by ID or Name.
func GetByIDOrName(ctx context.Context, c Client, id, name string) (*resource.Domain, error) {

	_, err := uuid.Parse(id)
	if err == nil {
		return c.Get(ctx, id)
	}

	return c.Single(ctx, &client.DomainListOptions{Names: client.Filter{Values: []string{name}}})

}

// GenerateCreate generates the DomainCreate from an *DomainParameters.
func GenerateCreate(spec v1alpha1.DomainParameters) *resource.DomainCreate {
	// if external-name is not set, search by Name and Space
	create := &resource.DomainCreate{}
	create.Name = spec.Name

	// RouterGroup can only be set when internal is false
	if spec.Internal != nil && !*spec.Internal {
		create.Internal = spec.Internal
		if spec.RouterGroup != nil {
			create.RouterGroup = &resource.Relationship{GUID: *spec.RouterGroup}
		}
	}

	if spec.Org != nil {
		create.Relationships = &resource.DomainRelationships{Organization: &resource.ToOneRelationship{Data: &resource.Relationship{GUID: *spec.Org}}}
	}

	if spec.SharedOrgs != nil {
		sharedOrgs := make([]string, len(spec.SharedOrgs))
		for i, org := range spec.SharedOrgs {
			sharedOrgs[i] = *org
		}
		create.Relationships.SharedOrganizations = resource.NewToManyRelationships(sharedOrgs)
	}

	// TODO: ADD labels and annotations
	return create
}

// GenerateObservation takes an Domain resource and returns *DomainObservation.
func GenerateObservation(o *resource.Domain) v1alpha1.DomainObservation {
	obs := v1alpha1.DomainObservation{
		ID:        ptr.To(o.GUID),
		CreatedAt: ptr.To(o.CreatedAt.Format(time.RFC3339)),
		UpdatedAt: ptr.To(o.UpdatedAt.Format(time.RFC3339)),
		Internal:  &o.Internal,
		Name:      ptr.To(o.Name),
	}

	if o.Relationships.SharedOrganizations != nil && len(o.Relationships.SharedOrganizations.Data) > 0 {
		obs.SharedOrgs = convertToStringPtrSlice(o.Relationships.SharedOrganizations.Data)
	}

	if o.RouterGroup != nil {
		obs.RouterGroup = ptr.To(o.RouterGroup.GUID)
	}

	if o.Metadata != nil {
		obs.Annotations = o.Metadata.Annotations
		obs.Labels = o.Metadata.Labels
	}

	return obs
}

// GenerateUpdate generates the Domain from an *DomainParameters. There is not really an option to update besides labels and annotations
func GenerateUpdate(spec v1alpha1.DomainParameters) *resource.DomainUpdate {
	return &resource.DomainUpdate{
		Metadata: &resource.Metadata{Labels: spec.Labels, Annotations: spec.Annotations},
	}
}

// IsUpToDate checks whether current state is up-to-date compared to the given
// set of parameters.
//
//nolint:gocyclo
func IsUpToDate(spec v1alpha1.DomainParameters, observed *resource.Domain) bool {
	// domain update does not support rename and change of many attributes, and can only update labels and annotations. we can safely return true for now

	// if spec.Name != observed.Name {
	// 	return false
	// }

	// if spec.Internal != nil && *spec.Internal != observed.Internal {
	// 	return false
	// }

	// if spec.RouterGroup != nil && (observed.RouterGroup == nil || *spec.RouterGroup != observed.RouterGroup.GUID) {
	// 	return false
	// }

	// // Some domains are not organization-scoped, it returns Relationships.Organization.Data nil. Since update of org is support, we can omit this checks
	// // if spec.Org != nil && (observed.Relationships.Organization == nil || *spec.Org != observed.Relationships.Organization.Data.GUID) {
	// //		return false
	// //	}

	// if observed.Relationships.SharedOrganizations != nil && len(spec.SharedOrgs) != len(observed.Relationships.SharedOrganizations.Data) {
	// 	return false
	// }

	// for i, org := range spec.SharedOrgs {
	// 	if *org != observed.Relationships.SharedOrganizations.Data[i].GUID {
	// 		return false
	// 	}
	// }

	return true
}

// convertToStringPtrSlice converts a slice of resource.Relationship to a slice of string pointers.
func convertToStringPtrSlice(relationships []resource.Relationship) []*string {
	result := make([]*string, len(relationships))
	for i, relationship := range relationships {
		result[i] = ptr.To(relationship.GUID)
	}
	return result
}

// LateInitialize fills the unassigned fields with values from a Domain resource.
func LateInitialize() bool {
	// Do nothing yet
	return false
}
