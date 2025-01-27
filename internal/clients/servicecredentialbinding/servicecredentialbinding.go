package servicecredentialbinding

import (
	"context"
	"encoding/json"
	"reflect"

	cfclient "github.com/cloudfoundry/go-cfclient/v3/client"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/google/uuid"
	"k8s.io/utils/ptr"

	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/resources/v1alpha2"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/clients"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/clients/job"
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

// NewListOptions generates ServiceCredentialBindingListOptions according to CR's ForProvider spec
func NewListOptions(spec v1alpha2.ServiceCredentialBindingParameters) *client.ServiceCredentialBindingListOptions {
	// if external-name is not set, search by Name and Space
	opt := client.NewServiceCredentialBindingListOptions()
	opt.Type.EqualTo(spec.Type)
	opt.Names.EqualTo(ptr.Deref(spec.Name, ""))

	if spec.ServiceInstance != nil {
		opt.ServiceInstanceGUIDs.EqualTo(ptr.Deref(spec.ServiceInstance, ""))
	}
	if spec.App != nil {
		opt.AppGUIDs.EqualTo(ptr.Deref(spec.App, ""))
	}
	return opt
}

// NewCreateOption generates ServiceCredentialBindingCreate according to CR's ForProvider spec
func NewCreateOption(spec v1alpha2.ServiceCredentialBindingParameters) *resource.ServiceCredentialBindingCreate {

	if spec.ServiceInstance == nil {
		return nil
	}

	var opt *resource.ServiceCredentialBindingCreate
	if spec.Type == "key" {
		opt = resource.NewServiceCredentialBindingCreateKey(*spec.ServiceInstance, *spec.Name)
	} else {
		opt = resource.NewServiceCredentialBindingCreateApp(*spec.ServiceInstance, *spec.App)
	}

	opt.WithName(*spec.Name)

	if spec.Parameters != nil {
		opt.WithJSONParameters(string(spec.Parameters.Raw))
	}
	return opt
}

// GetByIDOrSearch returns a ServiceCredentialBinding resource by guid or by spec
func GetByIDOrSearch(ctx context.Context, scbClient ServiceCredentialBinding, guid string, forProvider v1alpha2.ServiceCredentialBindingParameters) (*resource.ServiceCredentialBinding, error) {
	var r *resource.ServiceCredentialBinding

	err := uuid.Validate(guid)
	if err != nil { // guid is not a valid UUID, do a search.
		r, err = scbClient.Single(ctx, NewListOptions(forProvider))
	} else { // guid is a valid UUID
		r, err = scbClient.Get(ctx, guid) // do a get first
		if clients.ErrorIsNotFound(err) { // if not found, do a search.
			r, err = scbClient.Single(ctx, NewListOptions(forProvider))
		}
	}
	return r, err
}

// Create creates a ServiceCredentialBinding resource
func Create(ctx context.Context, scbClient ServiceCredentialBinding, spec v1alpha2.ServiceCredentialBindingParameters) (*resource.ServiceCredentialBinding, error) {
	opt := NewCreateOption(spec)

	// usually the binding is not ready yet at this point and is empty
	jobGUID, binding, err := scbClient.Create(ctx, opt)
	if err != nil {
		return binding, err
	}

	if jobGUID != "" { // async creation waits for the job to complete
		if err := job.PollJobComplete(ctx, scbClient, jobGUID); err != nil {
			return nil, err
		}

		// get the binding after the job is completed
		lo := cfclient.NewServiceCredentialBindingListOptions()

		lo.ServiceInstanceGUIDs.EqualTo(*spec.ServiceInstance)
		lo.Names.EqualTo(*spec.Name)

		return scbClient.Single(ctx, lo)
	}

	return binding, nil
}

// UpdateObservation updates the CR's AtProvider status from the observed resource
func UpdateObservation(observation *v1alpha2.ServiceCredentialBindingObservation, r *resource.ServiceCredentialBinding) {
	observation.ID = &r.Resource.GUID
	observation.LastOperation = &v1alpha2.LastOperation{
		Type:        r.LastOperation.Type,
		State:       r.LastOperation.State,
		Description: r.LastOperation.Description,
		UpdatedAt:   r.LastOperation.UpdatedAt.String(),
		CreatedAt:   r.LastOperation.CreatedAt.String(),
	}
}

// IsUpToDate checks whether the CR is up to date with the observed resource
func IsUpToDate(ctx context.Context, scbClient ServiceCredentialBinding, spec v1alpha2.ServiceCredentialBindingParameters, r resource.ServiceCredentialBinding) bool {
	if ptr.Deref(spec.Name, "") != ptr.Deref(r.Name, "") {
		return false
	}

	if spec.Parameters != nil {
		appliedParameters, err := scbClient.GetParameters(ctx, r.GUID)
		if err != nil {
			return false
		}

		var specParams map[string]string
		if err := json.Unmarshal(spec.Parameters.Raw, &specParams); err != nil {
			return false
		}
		return reflect.DeepEqual(appliedParameters, specParams)
	}
	return true
}

// GetBindingDetails returns the connection details of the ServiceCredentialBinding details
func GetBindingDetails(ctx context.Context, scbClient ServiceCredentialBinding, guid string, asJSON bool) managed.ConnectionDetails {
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
