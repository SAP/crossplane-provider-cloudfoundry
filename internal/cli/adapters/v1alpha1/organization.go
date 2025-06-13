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
	res "github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/resource"
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

func (a *CFOrganizationAdapter) FetchResources(ctx context.Context, filter res.ResourceFilter) ([]res.Resource, error) {
	// Get filter criteria
	criteria := filter.GetFilterCriteria()

	// Fetch resources from provider
	providerResources, err := a.GetResourcesByType(ctx, v1alpha1.Org_Kind, criteria)
	if err != nil {
		return nil, err
	}

	// Map to Resource interface
	resources := make([]res.Resource, len(providerResources))
	for i, providerResource := range providerResources {
		resource, err := a.MapToResource(providerResource, filter.GetManagementPolicies())
		if err != nil {
			return nil, err
		}
		resources[i] = resource
	}

	return resources, nil
}

func (a *CFOrganizationAdapter) MapToResource(providerResource interface{}, managementPolicies []v1.ManagementAction) (res.Resource, error) {
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

func (a *CFOrganizationAdapter) PreviewResource(resource res.Resource) {
	organization, ok := resource.(*CFOrganization)
	if !ok {
		fmt.Println("Invalid resource type provided for preview.")
		return
	}

	const (
		keyColor   = "\033[36m" // Cyan
		valueColor = "\033[32m" // Green
		resetColor = "\033[0m"  // Reset
	)

	fmt.Printf("%sapiVersion%s: %s%s%s\n", keyColor, resetColor, valueColor, organization.managedResource.APIVersion, resetColor)
	fmt.Printf("%skind%s: %s%s%s\n", keyColor, resetColor, valueColor, organization.managedResource.Kind, resetColor)
	fmt.Printf("%smetadata%s:\n  %sname%s: %s<generated on creation>%s\n", keyColor, resetColor, keyColor, resetColor, valueColor, resetColor)
	fmt.Printf("  %sannotations%s:\n    %scrossplane.io/external-name%s: %s%s%s\n", keyColor, resetColor, keyColor, resetColor, valueColor, organization.managedResource.Annotations["crossplane.io/external-name"], resetColor)
	fmt.Printf("%sspec%s:\n", keyColor, resetColor)
	fmt.Printf("  %sforProvider%s:\n", keyColor, resetColor)
	fmt.Printf("    %sname%s: %s%s%s\n", keyColor, resetColor, valueColor, organization.managedResource.Spec.ForProvider.Name, resetColor)
	fmt.Printf("  %smanagementPolicies%s:\n", keyColor, resetColor)
	for _, policy := range organization.managedResource.Spec.ManagementPolicies {
		fmt.Printf("    - %s%s%s\n", valueColor, policy, resetColor)
	}
	fmt.Println("---")
}
