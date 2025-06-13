package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/pkg/utils"
	res "github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/resource"
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
func (a *CFOrgMembersAdapter) FetchResources(ctx context.Context, filter res.ResourceFilter) ([]res.Resource, error) {
	// Get filter criteria
	criteria := filter.GetFilterCriteria()

	// Fetch resources from provider
	providerResources, err := a.GetResourcesByType(ctx, v1alpha1.OrgMembersKind, criteria)
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

// MapToResource converts a provider resource into a Resource interface
func (a *CFOrgMembersAdapter) MapToResource(providerResource interface{}, managementPolicies []v1.ManagementAction) (res.Resource, error) {
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
func (a *CFOrgMembersAdapter) PreviewResource(resource res.Resource) {
	members, ok := resource.(*CFOrgMembers)
	if !ok {
		fmt.Println("Invalid resource type provided for preview.")
		return
	}

	const maxWidth = 30

	utils.PrintLine("API Version", members.managedResource.APIVersion, maxWidth)
	utils.PrintLine("Kind", members.managedResource.Kind, maxWidth)
	utils.PrintLine("Name", "<generated on creation>", maxWidth)
	utils.PrintLine("Role Type", members.managedResource.Spec.ForProvider.RoleType, maxWidth)
	utils.PrintLine("Org", *members.managedResource.Spec.ForProvider.Org, maxWidth)

	// Print members
	memberStrings := make([]string, len(members.managedResource.Spec.ForProvider.Members))
	for i, member := range members.managedResource.Spec.ForProvider.Members {
		memberStrings[i] = fmt.Sprintf("%s (%s)", member.Username, member.Origin)
	}
	utils.PrintLine("Members", strings.Join(memberStrings, ", "), maxWidth)

	managementPolicies := make([]string, len(members.managedResource.Spec.ManagementPolicies))
	for i, policy := range members.managedResource.Spec.ManagementPolicies {
		managementPolicies[i] = string(policy)
	}
	utils.PrintLine("Management Policies", strings.Join(managementPolicies, ", "), maxWidth)

	fmt.Println(strings.Repeat("-", 80))
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
