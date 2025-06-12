package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/pkg/utils"
	res "github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/resource"
)

// CFSpace implements the Resource interface
type CFSpace struct {
	managedResource *v1alpha1.Space
	externalID      string
}

func (d *CFSpace) GetExternalID() string {
	return d.externalID
}

func (d *CFSpace) GetResourceType() string {
	return v1alpha1.Space_Kind
}

func (d *CFSpace) GetManagedResource() resource.Managed {
	return d.managedResource
}

func (d *CFSpace) SetProviderConfigReference(ref *v1.Reference) {
	d.managedResource.Spec.ProviderConfigReference = ref
}

func (d *CFSpace) SetManagementPolicies(policies []v1.ManagementAction) {
	d.managedResource.Spec.ManagementPolicies = policies
}

// CFSpaceAdapter implements the ResourceAdapter interface
type CFSpaceAdapter struct {
	BaseAdapter
}

func (a *CFSpaceAdapter) GetResourceType() string {
	return v1alpha1.Space_Kind
}

var sshEnabled bool

func (a *CFSpaceAdapter) FetchResources(ctx context.Context, filter res.ResourceFilter) ([]res.Resource, error) {
	// Get filter criteria
	criteria := filter.GetFilterCriteria()

	// Fetch resources from provider
	providerResources, err := a.GetResourcesByType(ctx, v1alpha1.Space_Kind, criteria)
	if err != nil {
		return nil, err
	}

	// Map to Resource interface
	resources := make([]res.Resource, len(providerResources))

	for i, providerResource := range providerResources {
		resourceData, ok := providerResource.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid provider resource format")
		}

		resource, err := a.MapToResource(resourceData["result"], filter.GetManagementPolicies())
		if ssh, ok := resourceData["SSH"].(bool); ok {
			sshEnabled = ssh
		} else {
			return nil, fmt.Errorf("invalid type for SSH field, expected bool")
		}
		if err != nil {
			return nil, err
		}
		resources[i] = resource
	}

	return resources, nil
}

func (a *CFSpaceAdapter) MapToResource(providerResource interface{}, managementPolicies []v1.ManagementAction) (res.Resource, error) {
	space, ok := providerResource.(*cfresource.Space)

	fmt.Println("- Space: " + space.Name + " with GUID: " + space.GUID)
	if !ok {
		return nil, fmt.Errorf("invalid resource type")
	}

	// Map resource
	managedResource := &v1alpha1.Space{}
	managedResource.APIVersion = schema.GroupVersion{Group: v1alpha1.CRDGroup, Version: v1alpha1.CRDVersion}.String()
	managedResource.Kind = v1alpha1.Space_Kind
	managedResource.SetAnnotations(map[string]string{"crossplane.io/external-name": space.GUID})
	managedResource.SetGenerateName(utils.NormalizeToRFC1123(space.Name))

	managedResource.Labels = map[string]string{
		"cf-name": space.Name,
	}

	// Set spec fields
	managedResource.Spec.ForProvider.Labels = space.Metadata.Labels
	managedResource.Spec.ForProvider.Annotations = space.Metadata.Annotations
	managedResource.Spec.ForProvider.Name = space.Name
	managedResource.Spec.ForProvider.AllowSSH = sshEnabled
	managedResource.Spec.ForProvider.Org = &space.Relationships.Organization.Data.GUID
	managedResource.Spec.ManagementPolicies = managementPolicies

	return &CFSpace{
		managedResource: managedResource,
		externalID:      space.GUID,
	}, nil
}

func (a *CFSpaceAdapter) PreviewResource(resource res.Resource) {
	space, ok := resource.(*CFSpace)
	if !ok {
		return
	}

	const maxWidth = 30

	utils.PrintLine("API Version", space.managedResource.APIVersion, maxWidth)
	utils.PrintLine("Kind", space.managedResource.Kind, maxWidth)
	utils.PrintLine("Name", "<generated on creation>", maxWidth)
	utils.PrintLine("External Name", space.managedResource.Annotations["crossplane.io/external-name"], maxWidth)

	allowSSH := "false"
	if space.managedResource.Spec.ForProvider.AllowSSH {
		allowSSH = "true"
	}
	utils.PrintLine("Allow SSH", allowSSH, maxWidth)

	if space.managedResource.Spec.ForProvider.Org != nil {
		utils.PrintLine("Organization GUID", *space.managedResource.Spec.ForProvider.Org, maxWidth)
	} else {
		utils.PrintLine("Organization GUID", "Not specified", maxWidth)
	}

	if len(space.managedResource.Spec.ForProvider.Labels) > 0 {
		var labels []string
		for key, value := range space.managedResource.Spec.ForProvider.Labels {
			labels = append(labels, fmt.Sprintf("%s: %s", key, *value))
		}
		utils.PrintLine("Labels", strings.Join(labels, "; "), maxWidth)
	} else {
		utils.PrintLine("Labels", "None", maxWidth)
	}

	if len(space.managedResource.Spec.ManagementPolicies) > 0 {
		var policies []string
		for _, policy := range space.managedResource.Spec.ManagementPolicies {
			policies = append(policies, string(policy))
		}
		utils.PrintLine("Management Policies", strings.Join(policies, ", "), maxWidth)
	} else {
		utils.PrintLine("Management Policies", "None", maxWidth)
	}

	fmt.Println(strings.Repeat("-", 80))
}
