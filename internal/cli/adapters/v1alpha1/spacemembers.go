package v1alpha1

import (
	"context"
	"fmt"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/pkg/utils"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/provider"
)

// CFSpaceMembersAdapter implements the ResourceAdapter interface
type CFSpaceMembersAdapter struct {
	BaseAdapter
}

// GetResourceType returns the resource type for SpaceMembers
func (a *CFSpaceMembersAdapter) GetResourceType() string {
	return v1alpha1.SpaceMembersKind
}

// FetchResources fetches SpaceMembers resources based on the provided filter criteria
func (a *CFSpaceMembersAdapter) FetchResources(ctx context.Context, filter provider.ResourceFilter) ([]provider.Resource, error) {
	// Get filter criteria
	criteria := filter.GetFilterCriteria()

	// Fetch resources from provider
	providerResources, err := a.CFClient.GetResourcesByType(ctx, v1alpha1.SpaceMembersKind, criteria)
	if err != nil {
		return nil, err
	}

	// Map to Resource interface
	resources := make([]provider.Resource, len(providerResources))
	for i, providerResource := range providerResources {
		resource, err := a.MapToResource(ctx, providerResource, filter.GetManagementPolicies())
		if err != nil {
			return nil, err
		}
		resources[i] = resource
	}

	return resources, nil
}

// MapToResource converts a provider resource into a Resource interface
func (a *CFSpaceMembersAdapter) MapToResource(ctx context.Context, providerResource interface{}, managementPolicies []v1.ManagementAction) (provider.Resource, error) {
	pr, ok := providerResource.(v1alpha1.SpaceMembersParameters)
	if !ok {
		return nil, fmt.Errorf("invalid provider resource type for space members")
	}
	name := *pr.SpaceName + "-" + pr.RoleType

	// Create the managed resource
	managedResource := &v1alpha1.SpaceMembers{}
	managedResource.APIVersion = schema.GroupVersion{Group: v1alpha1.CRDGroup, Version: v1alpha1.CRDVersion}.String()
	managedResource.Kind = v1alpha1.SpaceMembersKind
	managedResource.SetGenerateName(utils.NormalizeToRFC1123(name))
	managedResource.Spec.ForProvider = pr

	return &CFSpaceMembers{
		managedResource: managedResource,
		externalID:      name,
	}, nil
}

// PreviewResource displays the resource details in a formatted output
func (a *CFSpaceMembersAdapter) PreviewResource(resource provider.Resource) {
	preview(resource)
}

// CFSpaceMembers implements the Resource interface
type CFSpaceMembers struct {
	managedResource *v1alpha1.SpaceMembers
	externalID      string
}

func (d *CFSpaceMembers) GetExternalID() string {
	return d.externalID
}

func (d *CFSpaceMembers) GetResourceType() string {
	return v1alpha1.SpaceMembersKind
}

func (d *CFSpaceMembers) GetManagedResource() resource.Managed {
	return d.managedResource
}

func (d *CFSpaceMembers) SetProviderConfigReference(ref *v1.Reference) {
	d.managedResource.Spec.ProviderConfigReference = ref
}

func (d *CFSpaceMembers) SetManagementPolicies(policies []v1.ManagementAction) {
	d.managedResource.Spec.ManagementPolicies = policies
}
