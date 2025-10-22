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

// CFOrgMembersAdapter implements the ResourceAdapter interface
type CFOrgMembersAdapter struct {
	BaseAdapter
}

// GetResourceType returns the resource type for OrgMembers
func (a *CFOrgMembersAdapter) GetResourceType() string {
	return v1alpha1.OrgMembersKind
}

// FetchResources fetches OrgMembers resources based on the provided filter criteria
func (a *CFOrgMembersAdapter) FetchResources(ctx context.Context, filter provider.ResourceFilter) ([]provider.Resource, error) {
	// Get filter criteria
	criteria := filter.GetFilterCriteria()

	// Fetch resources from provider
	providerResources, err := a.CFClient.GetResourcesByType(ctx, v1alpha1.OrgMembersKind, criteria)
	if err != nil {
		return nil, err
	}

	// Map org members to resources
	resources := make([]provider.Resource, len(providerResources))
	for i, orgMember := range providerResources {
		resource, err := a.MapToResource(ctx, orgMember, filter.GetManagementPolicies())
		if err != nil {
			return nil, err
		}
		resources[i] = resource
	}

	return resources, nil
}

// MapToResource converts a provider resource into a Resource interface
func (a *CFOrgMembersAdapter) MapToResource(ctx context.Context, providerResource interface{}, managementPolicies []v1.ManagementAction) (provider.Resource, error) {
	pr, ok := providerResource.(v1alpha1.OrgMembersParameters)
	if !ok {
		return nil, fmt.Errorf("invalid provider resource type for org members")
	}
	name := *pr.OrgName + "-" + pr.RoleType
	// Create the managed resource
	managedResource := &v1alpha1.OrgMembers{}
	managedResource.APIVersion = schema.GroupVersion{Group: v1alpha1.CRDGroup, Version: v1alpha1.CRDVersion}.String()
	managedResource.Kind = v1alpha1.OrgMembersKind
	managedResource.SetGenerateName(utils.NormalizeToRFC1123(name))
	managedResource.Spec.ForProvider = pr

	return &CFOrgMembers{
		managedResource: managedResource,
		externalID:      name,
	}, nil
}

// PreviewResource displays the resource details in a formatted output
func (a *CFOrgMembersAdapter) PreviewResource(resource provider.Resource) {
	members, ok := resource.(*CFOrgMembers)
	if !ok {
		fmt.Println("Invalid resource type provided for preview.")
		return
	}

	const (
		keyColor   = "\033[36m" // Cyan
		valueColor = "\033[32m" // Green
		resetColor = "\033[0m"  // Reset
	)

	fmt.Printf("%sapiVersion%s: %s%s%s\n", keyColor, resetColor, valueColor, members.managedResource.APIVersion, resetColor)
	fmt.Printf("%skind%s: %s%s%s\n", keyColor, resetColor, valueColor, members.managedResource.Kind, resetColor)
	fmt.Printf("%smetadata%s:\n  %sname%s: %s<generated on creation>%s\n", keyColor, resetColor, keyColor, resetColor, valueColor, resetColor)
	fmt.Printf("%sspec%s:\n", keyColor, resetColor)
	fmt.Printf("  %sforProvider%s:\n", keyColor, resetColor)
	fmt.Printf("    %sroleType%s: %s%s%s\n", keyColor, resetColor, valueColor, members.managedResource.Spec.ForProvider.RoleType, resetColor)
	fmt.Printf("    %sorg%s: %s%s%s\n", keyColor, resetColor, valueColor, *members.managedResource.Spec.ForProvider.Org, resetColor)
	fmt.Printf("    %smembers%s:\n", keyColor, resetColor)
	for _, member := range members.managedResource.Spec.ForProvider.Members {
		fmt.Printf("      - %susername%s: %s%s%s\n", keyColor, resetColor, valueColor, member.Username, resetColor)
		fmt.Printf("        %sorigin%s: %s%s%s\n", keyColor, resetColor, valueColor, member.Origin, resetColor)
	}
	fmt.Printf("  %smanagementPolicies%s:\n", keyColor, resetColor)
	for _, policy := range members.managedResource.Spec.ManagementPolicies {
		fmt.Printf("    - %s%s%s\n", valueColor, policy, resetColor)
	}
	fmt.Println("---")
}

// CFOrgMembers implements the Resource interface
type CFOrgMembers struct {
	managedResource *v1alpha1.OrgMembers
	externalID      string
}

func (d *CFOrgMembers) GetExternalID() string {
	return d.externalID
}

func (d *CFOrgMembers) GetResourceType() string {
	return v1alpha1.OrgMembersKind
}

func (d *CFOrgMembers) GetManagedResource() resource.Managed {
	return d.managedResource
}

func (d *CFOrgMembers) SetProviderConfigReference(ref *v1.Reference) {
	d.managedResource.Spec.ProviderConfigReference = ref
}

func (d *CFOrgMembers) SetManagementPolicies(policies []v1.ManagementAction) {
	d.managedResource.Spec.ManagementPolicies = policies
}
