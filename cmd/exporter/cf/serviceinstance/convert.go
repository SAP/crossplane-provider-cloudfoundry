package serviceinstance

import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/cache"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/space"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/erratt"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/yaml"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

type serviceInstanceWithComment struct {
	*v1alpha1.ServiceInstance
	*cache.ResourceWithComment
}

var _ yaml.CommentedYAML = &serviceInstanceWithComment{}

func convertServiceInstanceTags(tags []string) []*string {
	result := make([]*string, len(tags))
	for i := range tags {
		result[i] = &tags[i]
	}
	return result
}

func generateServicePlan(ctx context.Context, cfClient *client.Client, serviceInstance *resource.ServiceInstance, evHandler export.EventHandler) *v1alpha1.ServicePlanParameters {
	if serviceInstance.Relationships.ServicePlan != nil &&
		serviceInstance.Relationships.ServicePlan.Data != nil {
		sPlan, err := cfClient.ServicePlans.Get(ctx, serviceInstance.Relationships.ServicePlan.Data.GUID)
		if err != nil {
			evHandler.Warn(erratt.Errorf("cannot get service plan of service instance: %w", err).With("service-instance-guid", serviceInstance.GUID, "service-plan-guid", serviceInstance.Relationships.ServicePlan.Data.GUID))
			return nil
		}
		sOffering, err := cfClient.ServiceOfferings.Get(ctx, sPlan.Relationships.ServiceOffering.Data.GUID)
		if err != nil {
			evHandler.Warn(erratt.Errorf("cannot get service offering of service plan: %w", err).With("service-ofering-guid", sPlan.Relationships.ServiceOffering.Data.GUID, "service-plan-guid", serviceInstance.Relationships.ServicePlan.Data.GUID))
			return nil
		}

		return &v1alpha1.ServicePlanParameters{
			ID:       ptr.To(serviceInstance.Relationships.ServicePlan.Data.GUID),
			Plan:     &sPlan.Name,
			Offering: &sOffering.Name,
		}
	}
	return nil
}

func generateCreds(ctx context.Context, cfClient *client.Client, serviceInstance *resource.ServiceInstance, evHandler export.EventHandler) *runtime.RawExtension {
	var jsonCredsBytes []byte

	if serviceInstance.Type != "managed" {
		creds, err := cfClient.ServiceInstances.GetUserProvidedCredentials(ctx, serviceInstance.GUID)
		if err != nil {
			evHandler.Warn(erratt.Errorf("cannot get service instance provided credentials: %w", err).With("guid", serviceInstance.GUID))
			return nil
		}
		jsonCredsBytes, err = creds.MarshalJSON()
		if err != nil {
			evHandler.Warn(erratt.Errorf("cannot JSON marshal service instance provided credentials: %w", err).With("guid", serviceInstance.GUID))
			return nil
		}
	}

	return &runtime.RawExtension{
		Raw: jsonCredsBytes,
	}
}

func generateParams(ctx context.Context, cfClient *client.Client, serviceInstance *resource.ServiceInstance, evHandler export.EventHandler) (*runtime.RawExtension, *string) {
	var jsonParams []byte
	var comment *string

	if serviceInstance.Type == "managed" {
		params, err := cfClient.ServiceInstances.GetManagedParameters(ctx, serviceInstance.GUID)
		if err == nil {
			jsonParams, err = params.MarshalJSON()
			if err != nil {
				evHandler.Warn(erratt.Errorf("cannot JSON marshal service instance managed parameters: %w", err).With("guid", serviceInstance.GUID))
				return nil, comment
			}
		} else {
			err = erratt.Errorf("cannot get service instance managed parameters: %w", err).With("serviceinstance-guid", serviceInstance.GUID)
			evHandler.Warn(err)
			comment = ptr.To(err.Error())
		}
	}
	re := &runtime.RawExtension{
		Raw: jsonParams,
	}
	return re, comment
}

func convertServiceInstanceResource(ctx context.Context, cfClient *client.Client, serviceInstance *res, evHandler export.EventHandler, resolveReferences bool) *serviceInstanceWithComment {
	slog.Debug("converting serviceInstance", "name", serviceInstance.Name)

	si := &serviceInstanceWithComment{
		ResourceWithComment: &cache.ResourceWithComment{},
	}
	si.CloneComment(serviceInstance.ResourceWithComment)

	servicePlan := generateServicePlan(ctx, cfClient, serviceInstance.ServiceInstance, evHandler)

	var maintenanceInfoDescription *string
	var maintenanceInfoVersion *string
	if mInfo := serviceInstance.MaintenanceInfo; mInfo != nil {
		maintenanceInfoDescription = ptr.To(mInfo.Description)
		maintenanceInfoVersion = ptr.To(mInfo.Description)
	}

	params, comment := generateParams(ctx, cfClient, serviceInstance.ServiceInstance, evHandler)
	if comment != nil {
		si.AddComment(*comment)
	}
	creds := generateCreds(ctx, cfClient, serviceInstance.ServiceInstance, evHandler)

	spaceReference := v1alpha1.SpaceReference{
		Space: &serviceInstance.Relationships.Space.Data.GUID,
	}

	if resolveReferences {
		if err := space.ResolveReference(ctx, cfClient, &spaceReference); err != nil {
			erra := erratt.Errorf("cannot resolve space reference: %w", err).With("serviceinstance-name", serviceInstance.GetName)
			evHandler.Warn(erra)
		}
	}

	si.ServiceInstance = &v1alpha1.ServiceInstance{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.ServiceInstance_Kind,
			APIVersion: v1alpha1.CRDGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceInstance.GetName(),
			Annotations: map[string]string{
				"crossplane.io/external-name": serviceInstance.GetGUID(),
			},
		},
		Spec: v1alpha1.ServiceInstanceSpec{
			ResourceSpec: v1.ResourceSpec{
				ManagementPolicies: []v1.ManagementAction{
					v1.ManagementActionObserve,
				},
			},
			ForProvider: v1alpha1.ServiceInstanceParameters{
				Name:           &serviceInstance.Name,
				Type:           v1alpha1.ServiceInstanceType(serviceInstance.Type),
				SpaceReference: spaceReference,
				Managed: v1alpha1.Managed{
					ServicePlan: servicePlan,
					Parameters:  params,
					// JSONParams: jsonParams,
					// ParametersSecretRef: &v1.SecretReference{},
					MaintenanceInfo: v1alpha1.MaintenanceInfo{
						Description: maintenanceInfoDescription,
						Version:     maintenanceInfoVersion,
					},
				},
				UserProvided: v1alpha1.UserProvided{
					Credentials: creds,
					// JSONCredentials: jsonCreds,
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
	return si
}
