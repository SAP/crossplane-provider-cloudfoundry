package v1alpha1

import (
	"context"
	"fmt"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/adapters"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/pkg/utils"
	res "github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/resource"
)

// CFServiceInstance implements the Resource interface
type CFServiceInstance struct {
	managedResource *v1alpha1.ServiceInstance
	externalID      string
}

func (d *CFServiceInstance) GetExternalID() string {
	return d.externalID
}

func (d *CFServiceInstance) GetResourceType() string {
	return v1alpha1.ServiceInstance_Kind
}

func (d *CFServiceInstance) GetManagedResource() resource.Managed {
	return d.managedResource
}

func (d *CFServiceInstance) SetProviderConfigReference(ref *v1.Reference) {
	d.managedResource.Spec.ProviderConfigReference = ref
}

func (d *CFServiceInstance) SetManagementPolicies(policies []v1.ManagementAction) {
	d.managedResource.Spec.ManagementPolicies = policies
}

// NewCFServiceInstanceAdapter creates a new CFServiceInstanceAdapter
func NewCFServiceInstanceAdapter(client adapters.CFClient) *CFServiceInstanceAdapter {
	return &CFServiceInstanceAdapter{
		BaseAdapter: BaseAdapter{CFClient: client},
	}
}

// CFServiceInstanceAdapter implements the ResourceAdapter interface
type CFServiceInstanceAdapter struct {
	BaseAdapter
}

func (a *CFServiceInstanceAdapter) GetResourceType() string {
	return v1alpha1.ServiceInstance_Kind
}

func (a *CFServiceInstanceAdapter) FetchResources(ctx context.Context, filter res.ResourceFilter) ([]res.Resource, error) {
	// Get filter criteria
	criteria := filter.GetFilterCriteria()

	// Fetch resources from provider
	providerResources, err := a.GetResourcesByType(ctx, v1alpha1.ServiceInstance_Kind, criteria)
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

func (a *CFServiceInstanceAdapter) MapToResource(providerResource interface{}, managementPolicies []v1.ManagementAction) (res.Resource, error) {
	serviceInstance, ok := providerResource.(*cfresource.ServiceInstance)

	fmt.Println("- Service Instance: " + serviceInstance.Name + " with GUID: " + serviceInstance.GUID)
	if !ok {
		return nil, fmt.Errorf("invalid resource type")
	}

	// Map resource
	managedResource := &v1alpha1.ServiceInstance{}
	managedResource.APIVersion = schema.GroupVersion{Group: v1alpha1.CRDGroup, Version: v1alpha1.CRDVersion}.String()
	managedResource.Kind = v1alpha1.ServiceInstance_Kind
	managedResource.SetAnnotations(map[string]string{"crossplane.io/external-name": serviceInstance.GUID})
	managedResource.SetGenerateName(utils.NormalizeToRFC1123(serviceInstance.Name))

	managedResource.Labels = map[string]string{
		"cf-name": serviceInstance.Name,
	}

	// Set spec fields
	managedResource.Spec.ManagementPolicies = managementPolicies
	managedResource.Spec.ForProvider.Annotations = serviceInstance.Metadata.Annotations
	managedResource.Spec.ForProvider.Name = &serviceInstance.Name
	managedResource.Spec.ForProvider.Space = &serviceInstance.Relationships.Space.Data.GUID
	// Define or retrieve the service type
	serviceType := v1alpha1.ServiceInstanceType(serviceInstance.Type)

	managedResource.Spec.ForProvider.Type = serviceType

	if serviceType == v1alpha1.ManagedService {
		if param, err := a.GetServiceCredentials(context.Background(), serviceInstance.GUID, serviceInstance.Type); err == nil && param != nil {
			managedResource.Spec.ForProvider.Parameters = &runtime.RawExtension{Raw: *param}
		}
		planID := serviceInstance.Relationships.ServicePlan.Data.GUID
		if sp, err := a.GetServicePlan(context.Background(), planID); err == nil && sp != nil {
			managedResource.Spec.ForProvider.ServicePlan = sp
		}

	}

	return &CFServiceInstance{
		managedResource: managedResource,
		externalID:      serviceInstance.GUID,
	}, nil
}

func (a *CFServiceInstanceAdapter) PreviewResource(resource res.Resource) {
	si, ok := resource.(*CFServiceInstance)
	if !ok {
		fmt.Println("Invalid resource type provided for preview.")
		return
	}

	const (
		keyColor   = "\033[36m" // Cyan
		valueColor = "\033[32m" // Green
		resetColor = "\033[0m"  // Reset
	)

	fmt.Printf("%sapiVersion%s: %s%s%s\n", keyColor, resetColor, valueColor, si.managedResource.APIVersion, resetColor)
	fmt.Printf("%skind%s: %s%s%s\n", keyColor, resetColor, valueColor, si.managedResource.Kind, resetColor)
	fmt.Printf("%smetadata%s:\n  %sname%s: %s<generated on creation>%s\n", keyColor, resetColor, keyColor, resetColor, valueColor, resetColor)
	fmt.Printf("  %sannotations%s:\n    %scrossplane.io/external-name%s: %s%s%s\n", keyColor, resetColor, keyColor, resetColor, valueColor, si.managedResource.Annotations["crossplane.io/external-name"], resetColor)
	fmt.Printf("%sspec%s:\n", keyColor, resetColor)
	fmt.Printf("  %sforProvider%s:\n", keyColor, resetColor)
	fmt.Printf("    %sname%s: %s%s%s\n", keyColor, resetColor, valueColor, si.managedResource.Spec.ForProvider.Name, resetColor)
	if si.managedResource.Spec.ForProvider.Space != nil {
		fmt.Printf("    %sspace%s: %s%s%s\n", keyColor, resetColor, valueColor, *si.managedResource.Spec.ForProvider.Space, resetColor)
	}
	if si.managedResource.Spec.ForProvider.ServicePlan != nil {
		fmt.Printf("    %sservicePlan%s: %s%s%s\n", keyColor, resetColor, valueColor, *si.managedResource.Spec.ForProvider.ServicePlan, resetColor)
	}
	fmt.Printf("  %smanagementPolicies%s:\n", keyColor, resetColor)
	for _, policy := range si.managedResource.Spec.ManagementPolicies {
		fmt.Printf("    - %s%s%s\n", valueColor, policy, resetColor)
	}
	fmt.Println("---")
}
