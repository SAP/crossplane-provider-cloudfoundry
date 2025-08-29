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
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/provider"
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

func (a *CFServiceInstanceAdapter) FetchResources(ctx context.Context, filter provider.ResourceFilter) ([]provider.Resource, error) {
	// Get filter criteria
	criteria := filter.GetFilterCriteria()

	// Fetch resources from provider
	providerResources, err := a.CFClient.GetResourcesByType(ctx, v1alpha1.ServiceInstance_Kind, criteria)
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

func (a *CFServiceInstanceAdapter) MapToResource(ctx context.Context, providerResource interface{}, managementPolicies []v1.ManagementAction) (provider.Resource, error) {
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
		if param, err := a.CFClient.GetServiceCredentials(ctx, serviceInstance.GUID, serviceInstance.Type); err == nil && param != nil {
			managedResource.Spec.ForProvider.Parameters = &runtime.RawExtension{Raw: *param}
		}
		planID := serviceInstance.Relationships.ServicePlan.Data.GUID
		if sp, err := a.CFClient.GetServicePlan(ctx, planID); err == nil && sp != nil {
			managedResource.Spec.ForProvider.ServicePlan = sp
		}

	}

	return &CFServiceInstance{
		managedResource: managedResource,
		externalID:      serviceInstance.GUID,
	}, nil
}

func (a *CFServiceInstanceAdapter) PreviewResource(resource provider.Resource) {
	preview(resource)
}
