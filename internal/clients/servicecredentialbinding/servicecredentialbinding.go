package servicecredentialbinding

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/google/uuid"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/job"
)

const (
	ErrServiceInstanceMissing = "ServiceInstance is required for key/app binding"
	ErrAppMissing             = "App is required for app binding"
	ErrNameMissing            = "Name is required for key binding"
	ErrBindingTypeUnknown     = "Unknown binding type. Supported types are key and app"
)

// serviceCredentialBinding defines interfaces to CloudFoundry ServiceCredentialBinding resource
type serviceCredentialBinding interface {
	Get(ctx context.Context, guid string) (*resource.ServiceCredentialBinding, error)
	GetDetails(ctx context.Context, guid string) (*resource.ServiceCredentialBindingDetails, error)
	GetParameters(ctx context.Context, guid string) (map[string]string, error)
	Single(ctx context.Context, opts *client.ServiceCredentialBindingListOptions) (*resource.ServiceCredentialBinding, error)
	Create(ctx context.Context, r *resource.ServiceCredentialBindingCreate) (string, *resource.ServiceCredentialBinding, error)
	Update(ctx context.Context, guid string, r *resource.ServiceCredentialBindingUpdate) (*resource.ServiceCredentialBinding, error)
	Delete(context.Context, string) (string, error)
}

// ServiceCredentialBinding defines interface to CloudFoundry ServiceCredentialBinding and async Job operation
type ServiceCredentialBinding interface {
	serviceCredentialBinding
	job.Job
}

// NewClient returns a new client using CloudFoundry base client
func NewClient(cfv3 *client.Client) ServiceCredentialBinding {
	return struct {
		serviceCredentialBinding
		job.Job
	}{cfv3.ServiceCredentialBindings, cfv3.Jobs}
}

// GetByIDOrSearch returns a ServiceCredentialBinding resource by guid or by spec
func GetByIDOrSearch(ctx context.Context, scbClient ServiceCredentialBinding, guid string, forProvider v1alpha1.ServiceCredentialBindingParameters) (*resource.ServiceCredentialBinding, error) {
	if err := uuid.Validate(guid); err != nil {
		opts, err := newListOptions(forProvider)
		if err != nil {
			return nil, err
		}
		return scbClient.Single(ctx, opts)
	}

	return scbClient.Get(ctx, guid)
}

// Create creates a ServiceCredentialBinding resource
func Create(ctx context.Context, scbClient ServiceCredentialBinding, forProvider v1alpha1.ServiceCredentialBindingParameters, params json.RawMessage) (*resource.ServiceCredentialBinding, error) {
	opt, err := newCreateOption(forProvider, params)
	if err != nil {
		return nil, err
	}

	// usually the binding is not ready yet at this point and is empty
	jobGUID, binding, err := scbClient.Create(ctx, opt)
	if err != nil {
		return binding, err
	}

	if jobGUID != "" { // async creation waits for the job to complete
		if err := job.PollJobComplete(ctx, scbClient, jobGUID); err != nil {
			return nil, err
		}
	}

	opts, err := newListOptions(forProvider)
	if err != nil {
		return nil, err
	}
	return scbClient.Single(ctx, opts)

}

// Update updates labels and annotations of a ServiceCredentialBinding resource
func Update(ctx context.Context, scbClient ServiceCredentialBinding, guid string, spec v1alpha1.ServiceCredentialBindingParameters) (*resource.ServiceCredentialBinding, error) {
	opt := newUpdateOption(spec)
	return scbClient.Update(ctx, guid, opt)
}

// Delete deletes a ServiceCredentialBinding resource
func Delete(ctx context.Context, scbClient ServiceCredentialBinding, guid string) error {
	_, err := scbClient.Delete(ctx, guid)
	return err
}

// GetConnectionDetails returns the connection details of the ServiceCredentialBinding details
func GetConnectionDetails(ctx context.Context, scbClient ServiceCredentialBinding, guid string, asJSON bool) managed.ConnectionDetails {
	bindingDetails, err := scbClient.GetDetails(ctx, guid)
	if err != nil {
		return nil
	}

	connectDetails := managed.ConnectionDetails{}
	if asJSON {
		jsonCredentials, err := json.Marshal(bindingDetails.Credentials)
		if err != nil {
			return nil
		}
		connectDetails["credentials"] = jsonCredentials
		return connectDetails
	}

	for key, value := range normalizeMap(bindingDetails.Credentials, make(map[string]string), "", "_") {
		connectDetails[key] = []byte(value)
	}

	return connectDetails
}

// newListOptions generates ServiceCredentialBindingListOptions according to CR's ForProvider spec
func newListOptions(spec v1alpha1.ServiceCredentialBindingParameters) (*client.ServiceCredentialBindingListOptions, error) {
	// if external-name is not set, search by Name and Space
	opt := client.NewServiceCredentialBindingListOptions()
	opt.Type.EqualTo(spec.Type)

	if spec.ServiceInstance == nil {
		return nil, errors.New(ErrServiceInstanceMissing)
	}
	opt.ServiceInstanceGUIDs.EqualTo(*spec.ServiceInstance)

	if spec.Type == "app" {
		if spec.App == nil {
			return nil, errors.New(ErrAppMissing)
		}
		opt.AppGUIDs.EqualTo(*spec.App)
	}

	if spec.Type == "key" {
		if spec.Name == nil {
			return nil, errors.New(ErrNameMissing)
		}
		opt.Names.EqualTo(*spec.Name)
	}

	return opt, nil
}

// newCreateOption generates ServiceCredentialBindingCreate according to CR's ForProvider spec
func newCreateOption(spec v1alpha1.ServiceCredentialBindingParameters, params json.RawMessage) (*resource.ServiceCredentialBindingCreate, error) {
	if spec.ServiceInstance == nil {
		return nil, errors.New(ErrServiceInstanceMissing)
	}

	var opt *resource.ServiceCredentialBindingCreate
	switch spec.Type {
	case "key":
		if spec.Name == nil {
			return nil, errors.New(ErrNameMissing)
		}

		opt = resource.NewServiceCredentialBindingCreateKey(*spec.ServiceInstance, *spec.Name)
	case "app":
		if spec.App == nil {
			return nil, errors.New(ErrAppMissing)
		}
		opt = resource.NewServiceCredentialBindingCreateApp(*spec.ServiceInstance, *spec.App)

		// for app binding, binding name is optional
		if spec.Name != nil {
			opt.WithName(*spec.Name)
		}
	default:
		return nil, errors.New(ErrBindingTypeUnknown)
	}

	if params != nil {
		opt.WithJSONParameters(string(params))
	}
	return opt, nil
}

// newUpdateOption generates ServiceCredentialBindingUpdate according to CR's ForProvider spec
func newUpdateOption(spec v1alpha1.ServiceCredentialBindingParameters) *resource.ServiceCredentialBindingUpdate {
	opt := &resource.ServiceCredentialBindingUpdate{}
	// TODO: implement update option. SCB support only updates for labels and annotations. No other fields can be updated. Labels and annotations are not supported yet, so for now we return an empty update option.
	return opt
}

// UpdateObservation updates the CR's AtProvider status from the observed resource
func UpdateObservation(observation *v1alpha1.ServiceCredentialBindingObservation, r *resource.ServiceCredentialBinding) {
	observation.GUID = r.Resource.GUID
	observation.LastOperation = &v1alpha1.LastOperation{
		Type:        r.LastOperation.Type,
		State:       r.LastOperation.State,
		Description: r.LastOperation.Description,
		UpdatedAt:   r.LastOperation.UpdatedAt.String(),
		CreatedAt:   r.LastOperation.CreatedAt.String(),
	}
}

// IsUpToDate checks whether the CR is up to date with the observed resource
func IsUpToDate(ctx context.Context, spec v1alpha1.ServiceCredentialBindingParameters, r resource.ServiceCredentialBinding) bool {
	// SCB support updates for labels and metadata only. This is to be implemented. For now return true
	return true
}
