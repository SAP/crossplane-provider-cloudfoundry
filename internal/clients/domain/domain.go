package domain

import (
	"context"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	xpresource "github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/google/uuid"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/job"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/metadata"
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

// ClientWrapper wraps the domain Client.
type ClientWrapper struct {
	Client
}

// FindDomainBySpec looks up a domain by name when external-name is empty.
// Name-only lookup: shared domains have no org, so org filter would exclude them.
func (c *ClientWrapper) FindDomainBySpec(ctx context.Context, spec v1alpha1.DomainParameters) (*resource.Domain, error) {
	opts := &client.DomainListOptions{
		Names: client.Filter{Values: []string{spec.Name}},
	}
	return c.Single(ctx, opts)
}

// GetDomainByGUID fetches a domain by its GUID.
func (c *ClientWrapper) GetDomainByGUID(ctx context.Context, guid string) (*resource.Domain, error) {
	return c.Get(ctx, guid)
}

// NewClient creates a new client instance from a cfclient.Domain instance.
func NewClient(cf *client.Client) (*ClientWrapper, job.Job) {
	return &ClientWrapper{Client: cf.Domains}, cf.Jobs
}

// Deprecated: Use FindDomainBySpec or GetDomainByGUID instead.
// GetByIDOrName returns a domain by ID or Name.
func GetByIDOrName(ctx context.Context, c Client, id, name string) (*resource.Domain, error) {

	_, err := uuid.Parse(id)
	if err == nil {
		return c.Get(ctx, id)
	}

	return c.Single(ctx, &client.DomainListOptions{Names: client.Filter{Values: []string{name}}})

}

// GenerateCreate generates the DomainCreate from an *DomainParameters.
func GenerateCreate(mg xpresource.Managed, spec v1alpha1.DomainParameters) *resource.DomainCreate {
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

	create.Metadata = metadata.BuildMetadata(mg, spec.Labels, spec.Annotations)
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
		obs.Labels = o.Metadata.Labels
		obs.Annotations = o.Metadata.Annotations
	}

	return obs
}

// GenerateUpdate generates the DomainUpdate from an *DomainParameters. There is not really an option to update besides labels and annotations
func GenerateUpdate(mg xpresource.Managed, spec v1alpha1.DomainParameters) *resource.DomainUpdate {
	return &resource.DomainUpdate{
		Metadata: metadata.BuildMetadata(mg, spec.Labels, spec.Annotations),
	}
}

// IsUpToDate checks whether current state is up-to-date compared to the given
// set of parameters.
//
//nolint:gocyclo
func IsUpToDate(mg xpresource.Managed, spec v1alpha1.DomainParameters, observed *resource.Domain) bool {
	if observed == nil {
		return false
	}
	desired := metadata.BuildMetadata(mg, spec.Labels, spec.Annotations)
	var observedLabels, observedAnnotations map[string]*string
	if observed.Metadata != nil {
		observedLabels = observed.Metadata.Labels
		observedAnnotations = observed.Metadata.Annotations
	}
	return metadata.IsMetadataUpToDate(desired.Labels, desired.Annotations, observedLabels, observedAnnotations)
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
