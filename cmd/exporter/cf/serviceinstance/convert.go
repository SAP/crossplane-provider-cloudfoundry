package serviceinstance

import (
	"context"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"
	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func convertServiceInstanceTags(tags []string) []*string {
	result := make([]*string, len(tags))
	for i := range tags {
		result[i] = &tags[i]
	}
	return result
}

func convertServiceInstanceResource(ctx context.Context, cfClient *client.Client, serviceInstance *resource.ServiceInstance, errChan chan<-erratt.ErrorWithAttrs) *v1alpha1.ServiceInstance {
	var servicePlan *v1alpha1.ServicePlanParameters
	if serviceInstance.Relationships.ServicePlan != nil &&
		serviceInstance.Relationships.ServicePlan.Data != nil {
		sPlan, err := cfClient.ServicePlans.Get(ctx, serviceInstance.Relationships.ServicePlan.Data.GUID)
		if err != nil {
			errChan <- erratt.Errorf("cannot get service plan of service instance: %w", err).With("service-instance-guid", serviceInstance.GUID, "service-plan-guid", serviceInstance.Relationships.ServicePlan.Data.GUID)
			return nil
		}
		sOffering, err := cfClient.ServiceOfferings.Get(ctx, sPlan.Relationships.ServiceOffering.Data.GUID)
		if err != nil {
			errChan <- erratt.Errorf("cannot get service offering of service plan: %w", err).With("service-ofering-guid", sPlan.Relationships.ServiceOffering.Data.GUID, "service-plan-guid", serviceInstance.Relationships.ServicePlan.Data.GUID)
			return nil
		}

		servicePlan = &v1alpha1.ServicePlanParameters{
			ID:       ptr.To(serviceInstance.Relationships.ServicePlan.Data.GUID),
			Plan:     &sPlan.Name,
			Offering: &sOffering.Name,
		}
	}
	var maintenanceInfoDescription *string
	var maintenanceInfoVersion *string
	if mInfo := serviceInstance.MaintenanceInfo; mInfo != nil {
		maintenanceInfoDescription = ptr.To(mInfo.Description)
		maintenanceInfoVersion = ptr.To(mInfo.Description)

	}
	var jsonParams *string
	var jsonCreds *string
	// var rawExtensionParams *runtime.RawExtension
	if serviceInstance.Type == "managed" {
		params, err := cfClient.ServiceInstances.GetManagedParameters(ctx, serviceInstance.GUID)
		if err == nil {
			// rawParams := &unstructured.Unstructured{}
			jsonParamsBytes, err := params.MarshalJSON()
			if err != nil {
				errChan <- erratt.Errorf("cannot JSON marshal service instance managed parameters: %w", err).With("guid", serviceInstance.GUID)
				return nil
			}
			jsonParams = ptr.To(string(jsonParamsBytes))
			// err = json.Unmarshal(jsonParamsBytes, &rawParams)
			// if err != nil {
			// 	return nil, erratt.Errorf("cannot JSON unmarshal service instance managed parameters: %w", err).With("guid", serviceInstance.GUID)
			// }
			// yamlBytes, err := yaml.Marshal(rawParams)
			// if err != nil {
			// 	return nil, erratt.Errorf("cannot YAML marshal service instance managed parameters: %w", err).With("guid", serviceInstance.GUID)
			// }

			// slog.Info("yaml conversion", "yamlBytes", string(yamlBytes))
			// rawExtensionParams = &runtime.RawExtension{
			// 	Raw: []byte("a: 1"),
			// }
		} else {
			errChan <- erratt.Errorf("cannot get service instance managed parameters: %w", err).With("serviceinstance-guid", serviceInstance.GUID)
			// return nil, erratt.Errorf("cannot get service instance managed parameters: %w", err).With("guid", serviceInstance.GUID)
		}
	} else {
		creds, err := cfClient.ServiceInstances.GetUserProvidedCredentials(ctx, serviceInstance.GUID)
		if err != nil {
			errChan <- erratt.Errorf("cannot get service instance provided credentials: %w", err).With("guid", serviceInstance.GUID)
			return nil
		}
		jsonCredsBytes, err := creds.MarshalJSON()
		if err != nil {
			errChan <- erratt.Errorf("cannot JSON marshal service instance provided credentials: %w", err).With("guid", serviceInstance.GUID)
			return nil
		}
		jsonCreds = ptr.To(string(jsonCredsBytes))
	}

	return &v1alpha1.ServiceInstance{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.ServiceInstance_Kind,
			APIVersion: v1alpha1.CRDGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceInstance.Name,
			Annotations: map[string]string{
				"crossplane.io/external-name": serviceInstance.GUID,
			},
		},
		Spec: v1alpha1.ServiceInstanceSpec{
			ResourceSpec: v1.ResourceSpec{
				ManagementPolicies: []v1.ManagementAction{
					v1.ManagementActionObserve,
				},
			},
			ForProvider: v1alpha1.ServiceInstanceParameters{
				Name: &serviceInstance.Name,
				Type: v1alpha1.ServiceInstanceType(serviceInstance.Type),
				SpaceReference: v1alpha1.SpaceReference{
					Space: &serviceInstance.Relationships.Space.Data.GUID,
				},
				Managed: v1alpha1.Managed{
					ServicePlan: servicePlan,
					// Parameters:          rawExtensionParams,
					JSONParams: jsonParams,
					// ParametersSecretRef: &v1.SecretReference{},
					MaintenanceInfo: v1alpha1.MaintenanceInfo{
						Description: maintenanceInfoDescription,
						Version:     maintenanceInfoVersion,
					},
				},
				UserProvided: v1alpha1.UserProvided{
					// Credentials:          &runtime.RawExtension{},
					JSONCredentials: jsonCreds,
					// CredentialsSecretRef: &v1.SecretReference{},
					RouteServiceURL: ptr.Deref(serviceInstance.RouteServiceURL, ""),
					SyslogDrainURL:  ptr.Deref(serviceInstance.SyslogDrainURL, ""),
				},
				// Timeouts:    v1alpha1.TimeoutsParameters{
				// 	Create: new(string),
				// 	Delete: new(string),
				// 	Update: new(string),
				// },
				Tags:        convertServiceInstanceTags(serviceInstance.Tags),
				Annotations: serviceInstance.Metadata.Annotations,
			},
		},
	}
}
