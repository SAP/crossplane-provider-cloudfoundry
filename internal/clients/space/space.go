package space

import (
	"context"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/org"
)

// Space is the interface that defines the methods that a Space client should implement.
type Space interface {
	Get(ctx context.Context, guid string) (*resource.Space, error)
	Single(ctx context.Context, opts *client.SpaceListOptions) (*resource.Space, error)
	Create(ctx context.Context, r *resource.SpaceCreate) (*resource.Space, error)
	Update(ctx context.Context, guid string, r *resource.SpaceUpdate) (*resource.Space, error)
	Delete(ctx context.Context, guid string) (string, error)
}

// Feature is the interface that defines the methods that a Feature client should implement.
type Feature interface {
	EnableSSH(ctx context.Context, spaceGUID string, enable bool) error
	IsSSHEnabled(ctx context.Context, spaceGUID string) (bool, error)
}

// NewClient creates a new cf client and return interfaces for Space and SpaceFeatures
func NewClient(cf *client.Client) (Space, Feature, org.Client) {

	return cf.Spaces, cf.SpaceFeatures, cf.Organizations
}

// GetByIDOrSpec retrieves a Space by its GUID or by its specification.
func GetByIDOrSpec(ctx context.Context, spaceClient Space, guid string, spec v1alpha1.SpaceParameters) (*resource.Space, error) {
	if clients.IsValidGUID(guid) {
		return spaceClient.Get(ctx, guid)
	}

	return spaceClient.Single(ctx, GenerateListOption(spec))
}

// GenerateListOption generates the list options for the client.
func GenerateListOption(spec v1alpha1.SpaceParameters) *client.SpaceListOptions {
	opts := &client.SpaceListOptions{
		ListOptions: nil,
	}

	opts.Names = client.Filter{Values: []string{spec.Name}}

	if spec.Org != nil {
		opts.OrganizationGUIDs = client.Filter{Values: []string{*spec.Org}}
	}

	return opts
}

// GenerateCreate generates the SpaceCreate from an *SpaceParameters
func GenerateCreate(spec v1alpha1.SpaceParameters) *resource.SpaceCreate {
	org := ptr.Deref(spec.Org, "")
	return resource.NewSpaceCreate(spec.Name, org)
}

// GenerateUpdate generates the SpaceCreate from an *SpaceParameters
func GenerateUpdate(spec v1alpha1.SpaceParameters) *resource.SpaceUpdate {
	return &resource.SpaceUpdate{
		Name:     spec.Name,
		Metadata: &resource.Metadata{},
	}
}

// GenerateObservation takes an Space resource and returns *SpaceObservation.
func GenerateObservation(o *resource.Space, ssh bool) v1alpha1.SpaceObservation {
	obs := v1alpha1.SpaceObservation{
		ID:        o.GUID,
		Name:      o.Name,
		Org:       o.Relationships.Organization.Data.GUID,
		AllowSSH:  ssh,
		CreatedAt: ptr.To(o.CreatedAt.Format(time.RFC3339)),
		UpdatedAt: ptr.To(o.UpdatedAt.Format(time.RFC3339)),
	}
	if o.Relationships.Quota != nil && o.Relationships.Quota.Data != nil {
		obs.Quota = ptr.To(o.Relationships.Quota.Data.GUID)
	}
	if o.Metadata != nil {
		obs.Annotations = o.Metadata.Annotations
		obs.Labels = o.Metadata.Labels
	}
	return obs
}

// LateInitialize fills the unassigned fields with values from a Space resource.
func LateInitialize(cr *v1alpha1.Space, from *resource.Space, ssh bool) bool {
	// nothing to late initialize
	return false
}

// IsUpToDate checks whether current state is up-to-date compared to the given
// set of parameters.
func IsUpToDate(spec v1alpha1.SpaceParameters, observed *resource.Space, ssh bool) bool {
	// rename or update ssh setting
	return spec.Name == observed.Name && (spec.AllowSSH == ssh)

}

// IsSSHEnabled checks whether SSH is enabled for the given space.
func IsSSHEnabled(ctx context.Context, f Feature, spaceGUID string) (bool, error) {
	return f.IsSSHEnabled(ctx, spaceGUID)
}
