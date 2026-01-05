package serviceroutebinding

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/uuid"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/job"
	cfclient "github.com/cloudfoundry/go-cfclient/v3/client"
	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"k8s.io/apimachinery/pkg/runtime"
)

type serviceRouteBinding interface {
	Get(ctx context.Context, guid string) (*resource.ServiceRouteBinding, error)
	Single(ctx context.Context, opts *client.ServiceRouteBindingListOptions) (*resource.ServiceRouteBinding, error)
	Create(ctx context.Context, r *resource.ServiceRouteBindingCreate) (string, *resource.ServiceRouteBinding, error)
	Update(ctx context.Context, guid string, r *resource.ServiceRouteBindingUpdate) (*resource.ServiceRouteBinding, error)
	Delete(context.Context, string) (string, error)
	GetParameters(ctx context.Context, guid string) (map[string]string, error)
}

type ServiceRouteBinding interface {
	serviceRouteBinding
	job.Job
}

// NewClient returns a new client using CloudFoundry base client
func NewClient(cfv3 *client.Client) ServiceRouteBinding {
	return struct {
		serviceRouteBinding
		job.Job
	}{cfv3.ServiceRouteBindings, cfv3.Jobs}
}

func GetByID(ctx context.Context, srbClient ServiceRouteBinding, guid string, forProvider v1alpha1.ServiceRouteBindingParameters) (*cfresource.ServiceRouteBinding, error) {

	if err := uuid.Validate(guid); err != nil {
		return nil, err
	}
	// try to find by GUID
	return srbClient.Get(ctx, guid)
}

func Create(ctx context.Context, srbClient ServiceRouteBinding, forProvider v1alpha1.ServiceRouteBindingParameters, parametersFromSecret runtime.RawExtension) (*resource.ServiceRouteBinding, error) {
	opt := newCreateOption(forProvider, parametersFromSecret)

	jobGUID, binding, err := srbClient.Create(ctx, opt)
	if err != nil {
		return binding, err
	}

	if jobGUID != "" { // async creation waits for the job to complete
		if err := job.PollJobComplete(ctx, srbClient, jobGUID); err != nil {
			return nil, err
		}
	}
	return srbClient.Single(ctx, createToListOptions(opt))
}

func newCreateOption(forProvider v1alpha1.ServiceRouteBindingParameters, parametersFromSecret runtime.RawExtension) *cfresource.ServiceRouteBindingCreate {
	creationPayload := cfresource.NewServiceRouteBindingCreate(forProvider.Route, forProvider.ServiceInstance)

	if forProvider.Parameters.Raw != nil {
		creationPayload.Parameters = (*json.RawMessage)(&forProvider.Parameters.Raw)
	} else if parametersFromSecret.Raw != nil {
		creationPayload.Parameters = (*json.RawMessage)(&parametersFromSecret.Raw)
	}
	return creationPayload
}

func createToListOptions(create *cfresource.ServiceRouteBindingCreate) *client.ServiceRouteBindingListOptions {
	opts := cfclient.NewServiceRouteBindingListOptions()
	opts.RouteGUIDs.EqualTo(create.Relationships.Route.Data.GUID)
	opts.ServiceInstanceGUIDs.EqualTo(create.Relationships.ServiceInstance.Data.GUID)
	return opts
}

func Update(ctx context.Context, srbClient ServiceRouteBinding, guid string, forProvider v1alpha1.ServiceRouteBindingParameters) (*resource.ServiceRouteBinding, error) {
	// currently not implemented, since CF only support update of labels/annotations for ServiceRouteBinding
	return srbClient.Update(ctx, guid, &cfresource.ServiceRouteBindingUpdate{})
}

func Delete(ctx context.Context, srbClient ServiceRouteBinding, guid string) error {
	jobGUID, err := srbClient.Delete(ctx, guid)
	if err != nil {
		return err
	}
	if jobGUID != "" {
		return job.PollJobComplete(ctx, srbClient, jobGUID)
	}
	return err
}

func UpdateObservation(observation *v1alpha1.ServiceRouteBindingObservation, r *resource.ServiceRouteBinding, externalParameters *runtime.RawExtension) {
	observation.GUID = r.GUID
	if !r.CreatedAt.IsZero() {
		formatted := r.CreatedAt.UTC().Format(time.RFC3339)
		observation.CreatedAt = &formatted
	}
	observation.LastOperation = &v1alpha1.LastOperation{
		Type:      r.LastOperation.Type,
		State:     r.LastOperation.State,
		CreatedAt: r.LastOperation.CreatedAt.String(),
		UpdatedAt: r.LastOperation.UpdatedAt.String(),
	}

	observation.Links = buildLinks(r.Links)
	if r.Metadata != nil && (r.Metadata.Labels != nil || r.Metadata.Annotations != nil) {
		observation.ResourceMetadata = v1alpha1.ResourceMetadata{
			Labels:      r.Metadata.Labels,
			Annotations: r.Metadata.Annotations,
		}
	}

	if r.Relationships.ServiceInstance.Data != nil {
		observation.ServiceInstance = r.Relationships.ServiceInstance.Data.GUID
	}
	if r.Relationships.Route.Data != nil {
		observation.Route = r.Relationships.Route.Data.GUID
	}
	observation.RouteServiceUrl = r.RouteServiceURL

	if externalParameters != nil {
		observation.Parameters = *externalParameters
	}
}

// builds links map from CF links
func buildLinks(cfLinks cfresource.Links) v1alpha1.Links {
	if cfLinks == nil {
		return v1alpha1.Links{}
	}
	links := make(v1alpha1.Links)
	for k, v := range cfLinks {
		l := v1alpha1.Link{Href: v.Href}
		if v.Method != "" {
			l.Method = &v.Method
		}
		links[k] = l
	}
	return links
}

func GetParameters(ctx context.Context, srbClient ServiceRouteBinding, guid string) (*runtime.RawExtension, error) {
	params, err := srbClient.GetParameters(ctx, guid)
	if err != nil {
		return nil, err
	}

	// Marshal map to JSON bytes
	jsonBytes, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	return &runtime.RawExtension{Raw: jsonBytes}, nil
}
