package v1alpha1

import (
	"context"
	"fmt"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	res "github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/resource"
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
type CFRouteAdapter struct {
	BaseAdapter
}

func (a *CFRouteAdapter) GetResourceType() string {
	return v1alpha1.RouteKind
}

func (a *CFRouteAdapter) FetchResources(ctx context.Context, filter res.ResourceFilter) ([]res.Resource, error) {
	// Get filter criteria
	criteria := filter.GetFilterCriteria()

	// Fetch resources from provider
	providerResources, err := a.GetResourcesByType(ctx, v1alpha1.RouteKind, criteria)
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
		if err != nil {
			return nil, err
		}
		resources[i] = resource
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
		fmt.Println("Invalid resource type provided for preview.")
		return
	}

	const (
		keyColor   = "\033[36m" // Cyan
		valueColor = "\033[32m" // Green
		resetColor = "\033[0m"  // Reset
	)

	fmt.Printf("%sapiVersion%s: %s%s%s\n", keyColor, resetColor, valueColor, route.managedResource.APIVersion, resetColor)
	fmt.Printf("%skind%s: %s%s%s\n", keyColor, resetColor, valueColor, route.managedResource.Kind, resetColor)
	fmt.Printf("%smetadata%s:\n  %sname%s: %s<generated on creation>%s\n", keyColor, resetColor, keyColor, resetColor, valueColor, resetColor)
	fmt.Printf("  %sannotations%s:\n    %scrossplane.io/external-name%s: %s%s%s\n", keyColor, resetColor, keyColor, resetColor, valueColor, route.managedResource.Annotations["crossplane.io/external-name"], resetColor)
	fmt.Printf("%sspec%s:\n", keyColor, resetColor)
	fmt.Printf("  %sforProvider%s:\n", keyColor, resetColor)
	fmt.Printf("    %sdomain%s: %s%s%s\n", keyColor, resetColor, valueColor, route.managedResource.Spec.ForProvider.Domain, resetColor)
	if route.managedResource.Spec.ForProvider.Space != nil {
		fmt.Printf("    %sspace%s: %s%s%s\n", keyColor, resetColor, valueColor, *route.managedResource.Spec.ForProvider.Space, resetColor)
	}
	if route.managedResource.Spec.ForProvider.Host != nil {
		fmt.Printf("    %shost%s: %s%s%s\n", keyColor, resetColor, valueColor, *route.managedResource.Spec.ForProvider.Host, resetColor)
	}
	if route.managedResource.Spec.ForProvider.Path != nil {
		fmt.Printf("    %spath%s: %s%s%s\n", keyColor, resetColor, valueColor, *route.managedResource.Spec.ForProvider.Path, resetColor)
	}
	fmt.Printf("  %smanagementPolicies%s:\n", keyColor, resetColor)
	for _, policy := range route.managedResource.Spec.ManagementPolicies {
		fmt.Printf("    - %s%s%s\n", valueColor, policy, resetColor)
	}
	fmt.Println("---")
}
