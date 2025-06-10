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

// CFRoute implements the Resource interface
type CFRoute struct {
	managedResource *v1alpha1.Route
	externalID      string
}

func (d *CFRoute) GetExternalID() string {
	return d.externalID
}

func (d *CFRoute) GetResourceType() string {
	return v1alpha1.RouteKind
}

func (d *CFRoute) GetManagedResource() resource.Managed {
	return d.managedResource
}

func (d *CFRoute) SetProviderConfigReference(ref *v1.Reference) {
	d.managedResource.Spec.ProviderConfigReference = ref
}

func (d *CFRoute) SetManagementPolicies(policies []v1.ManagementAction) {
	d.managedResource.Spec.ManagementPolicies = policies
}

// CFRouteAdapter implements the ResourceAdapter interface
type CFRouteAdapter struct{}

func (a *CFRouteAdapter) GetResourceType() string {
	return v1alpha1.RouteKind
}

func (a *CFRouteAdapter) FetchResources(ctx context.Context, client client.ProviderClient, filter res.ResourceFilter) ([]res.Resource, error) {
	// Get filter criteria
	criteria := filter.GetFilterCriteria()

	// Fetch resources from provider
	providerResources, err := client.GetResourcesByType(ctx, v1alpha1.RouteKind, criteria)
	if err != nil {
		return nil, err
	}

	// Map to Resource interface
	var resources []res.Resource

	for _, providerResource := range providerResources {
		resourceData, ok := providerResource.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid provider resource format")
		}

		resource, err := a.MapToResource(resourceData["result"], filter.GetManagementPolicies())
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}

	return resources, nil
}

func (a *CFRouteAdapter) MapToResource(providerResource interface{}, managementPolicies []v1.ManagementAction) (res.Resource, error) {
	route, ok := providerResource.(*cfresource.Route)

	fmt.Println("- Route: " + route.Host + " with GUID: " + route.GUID) // TODO what would happen if host was empty?
	if !ok {
		return nil, fmt.Errorf("invalid resource type")
	}

	// Map resource
	managedResource := &v1alpha1.Route{}
	managedResource.APIVersion = schema.GroupVersion{Group: v1alpha1.CRDGroup, Version: v1alpha1.CRDVersion}.String()
	managedResource.Kind = v1alpha1.RouteKind
	managedResource.SetAnnotations(map[string]string{"crossplane.io/external-name": route.GUID})

	// TODO Is it needed?
	// managedResource.Labels = map[string]string{
	// 	"cf-name": route.Name,
	// }

	// Set spec fields
	managedResource.Spec.ForProvider.Host = &route.Host
	managedResource.Spec.ForProvider.Path = &route.Path
	managedResource.Spec.ForProvider.Port = route.Port

	// TODO
	// managedResource.Spec.ForProvider.Options = ???

	managedResource.Spec.ForProvider.Space = &route.Relationships.Space.Data.GUID
	managedResource.Spec.ForProvider.Domain = &route.Relationships.Domain.Data.GUID

	managedResource.Spec.ManagementPolicies = managementPolicies

	return &CFRoute{
		managedResource: managedResource,
		externalID:      route.GUID,
	}, nil
}

func (a *CFRouteAdapter) PreviewResource(resource res.Resource) {
	route, ok := resource.(*CFRoute)
	if !ok {
		return
	}

	const maxWidth = 30

	utils.PrintLine("API Version", route.managedResource.APIVersion, maxWidth)
	utils.PrintLine("Kind", route.managedResource.Kind, maxWidth)
	utils.PrintLine("Name", "<generated on creation>", maxWidth)
	utils.PrintLine("External Name", route.managedResource.Annotations["crossplane.io/external-name"], maxWidth)

	// Space GUID
	if route.managedResource.Spec.ForProvider.Space != nil {
		utils.PrintLine("Space GUID", *route.managedResource.Spec.ForProvider.Space, maxWidth)
	} else {
		utils.PrintLine("Space GUID", "Not specified", maxWidth)
	}

	// Domain GUID
	if route.managedResource.Spec.ForProvider.Domain != nil {
		utils.PrintLine("Domain GUID", *route.managedResource.Spec.ForProvider.Domain, maxWidth)
	} else {
		utils.PrintLine("Domain GUID", "Not specified", maxWidth)
	}

	// all the other forProvider fields
	if route.managedResource.Spec.ForProvider.Host != nil {
		utils.PrintLine("Host GUID", *route.managedResource.Spec.ForProvider.Host, maxWidth)
	} else {
		utils.PrintLine("Host GUID", "Not specified", maxWidth)
	}

	if route.managedResource.Spec.ForProvider.Path != nil {
		utils.PrintLine("Path GUID", *route.managedResource.Spec.ForProvider.Path, maxWidth)
	} else {
		utils.PrintLine("Path GUID", "Not specified", maxWidth)
	}

	if route.managedResource.Spec.ForProvider.Port != nil {
		utils.PrintLine("Port GUID", fmt.Sprint(*route.managedResource.Spec.ForProvider.Port), maxWidth)
	} else {
		utils.PrintLine("Port GUID", "Not specified", maxWidth)
	}

	if len(route.managedResource.Spec.ManagementPolicies) > 0 {
		var policies []string
		for _, policy := range route.managedResource.Spec.ManagementPolicies {
			policies = append(policies, string(policy))
		}
		utils.PrintLine("Management Policies", strings.Join(policies, ", "), maxWidth)
	} else {
		utils.PrintLine("Management Policies", "None", maxWidth)
	}

	fmt.Println(strings.Repeat("-", 80))
}
