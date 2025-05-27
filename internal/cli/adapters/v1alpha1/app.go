package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/pkg/utils"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/client"
	res "github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/resource"
	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CFApp implements the Resource interface
type CFApp struct {
	managedResource *v1alpha1.App
	externalID      string
}

func (d *CFApp) GetExternalID() string {
	return d.externalID
}

func (d *CFApp) GetResourceType() string {
	return v1alpha1.App_Kind
}

func (d *CFApp) GetManagedResource() resource.Managed {
	return d.managedResource
}

func (d *CFApp) SetProviderConfigReference(ref *v1.Reference) {
	d.managedResource.Spec.ProviderConfigReference = ref
}

func (d *CFApp) SetManagementPolicies(policies []v1.ManagementAction) {
	d.managedResource.Spec.ManagementPolicies = policies
}

// CFSpaceAdapter implements the ResourceAdapter interface
type CFAppAdapter struct{}

func (a *CFAppAdapter) GetResourceType() string {
	return v1alpha1.App_Kind
}

func (a *CFAppAdapter) FetchResources(ctx context.Context, client client.ProviderClient, filter res.ResourceFilter) ([]res.Resource, error) {
	// Get filter criteria
	criteria := filter.GetFilterCriteria()

	// Fetch resources from provider
	providerResources, err := client.GetResourcesByType(ctx, v1alpha1.App_Kind, criteria)

	if err != nil {
		return nil, err
	}

	// Map to Resource interface
	var resources []res.Resource
	for _, providerResource := range providerResources {
		resource, err := a.MapToResource(providerResource, filter.GetManagementPolicies())
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}

	return resources, nil
}

func (a *CFAppAdapter) MapToResource(providerResource interface{}, managementPolicies []v1.ManagementAction) (res.Resource, error) {
	app, ok := providerResource.(*cfresource.App)
	
	fmt.Println("- App: " + app.Name + " with GUID: " + app.GUID)

	if !ok {
		return nil, fmt.Errorf("invalid resource type")
	}

	// Map resource
	managedResource := &v1alpha1.App{}
	managedResource.APIVersion = schema.GroupVersion{Group:   v1alpha1.CRDGroup,	Version: v1alpha1.CRDVersion}.String()
	managedResource.Kind = v1alpha1.App_Kind
	managedResource.SetAnnotations(map[string]string{"crossplane.io/external-name": app.GUID})
	managedResource.SetGenerateName(utils.NormalizeToRFC1123(app.Name))
	managedResource.Spec.ForProvider.Space = &app.Relationships.Space.Data.GUID
	managedResource.Spec.DeletionPolicy = "Orphan"

	managedResource.Labels = map[string]string{
		"cf-name": app.Name,
	}

	// Set spec fields
	managedResource.Spec.ForProvider.Name = app.Name
	managedResource.Spec.ManagementPolicies = managementPolicies
	

	return &CFApp{
		managedResource: managedResource,
		externalID:      app.GUID,
	}, nil
}

func (a *CFAppAdapter) PreviewResource(resource res.Resource) {
	app, ok := resource.(*CFApp)
	if !ok {
		fmt.Println("Invalid resource type provided for preview.")
		return
	}

	const maxWidth = 30

	utils.PrintLine("API Version", app.managedResource.APIVersion, maxWidth)
	utils.PrintLine("Kind", app.managedResource.Kind, maxWidth)
	utils.PrintLine("Name", "<generated on creation>", maxWidth)
	utils.PrintLine("External Name", app.managedResource.Annotations["crossplane.io/external-name"], maxWidth)

	var managementPolicies []string
	for _, policy := range app.managedResource.Spec.ManagementPolicies {
		managementPolicies = append(managementPolicies, string(policy))
	}
	utils.PrintLine("Management Policies", strings.Join(managementPolicies, ", "), maxWidth)

	fmt.Println(strings.Repeat("-", 80))
}
