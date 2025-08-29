package v1alpha1

import (
	"context"
	"fmt"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/pkg/utils"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/provider"
)

// CFOrganization implements the Resource interface
type CFOrganization struct {
	managedResource *v1alpha1.Organization
	externalID      string
}

func (d *CFOrganization) GetExternalID() string {
	return d.externalID
}

func (d *CFOrganization) GetResourceType() string {
	return v1alpha1.Org_Kind
}

func (d *CFOrganization) GetManagedResource() resource.Managed {
	return d.managedResource
}

func (d *CFOrganization) SetProviderConfigReference(ref *v1.Reference) {
	d.managedResource.Spec.ProviderConfigReference = ref
}

func (d *CFOrganization) SetManagementPolicies(policies []v1.ManagementAction) {
	d.managedResource.Spec.ManagementPolicies = policies
}

// CFOrgaizationAdapter implements the ResourceAdapter interface
type CFOrganizationAdapter struct {
	BaseAdapter
}

func (a *CFOrganizationAdapter) GetResourceType() string {
	return v1alpha1.Org_Kind
}

func (a *CFOrganizationAdapter) FetchResources(ctx context.Context, filter provider.ResourceFilter) ([]provider.Resource, error) {
	// Get filter criteria
	criteria := filter.GetFilterCriteria()

	// Fetch resources from provider
	providerResources, err := a.CFClient.GetResourcesByType(ctx, v1alpha1.Org_Kind, criteria)
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

func (a *CFOrganizationAdapter) MapToResource(ctx context.Context, providerResource interface{}, managementPolicies []v1.ManagementAction) (provider.Resource, error) {
	organization, ok := providerResource.(*cfresource.Organization)

	fmt.Println("- Org: " + organization.Name + " with GUID: " + organization.GUID)

	if !ok {
		return nil, fmt.Errorf("invalid resource type")
	}

	// Map resource
	managedResource := &v1alpha1.Organization{}
	managedResource.APIVersion = schema.GroupVersion{Group: v1alpha1.CRDGroup, Version: v1alpha1.CRDVersion}.String()
	managedResource.Kind = v1alpha1.Org_Kind
	managedResource.SetAnnotations(map[string]string{"crossplane.io/external-name": organization.Name})
	managedResource.SetGenerateName(utils.NormalizeToRFC1123(organization.Name))

	managedResource.Labels = map[string]string{
		"cf-name": organization.Name,
	}

	// Set spec fields
	managedResource.Spec.ForProvider.Name = organization.Name
	managedResource.Spec.ManagementPolicies = managementPolicies

	return &CFOrganization{
		managedResource: managedResource,
		externalID:      organization.GUID,
	}, nil
}

func (a *CFOrganizationAdapter) PreviewResource(resource provider.Resource) {
	preview(resource)
}
