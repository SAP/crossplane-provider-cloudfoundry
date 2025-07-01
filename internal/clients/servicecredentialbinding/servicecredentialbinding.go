package servicecredentialbinding

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"strings"
	"time"

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
func GetByIDOrSearch(ctx context.Context, scbClient ServiceCredentialBinding, guid string, cr v1alpha1.ServiceCredentialBinding) (*resource.ServiceCredentialBinding, error) {
	if err := uuid.Validate(guid); err != nil {
		opts, err := newListOptions(cr)
		if err != nil {
			return nil, err
		}
		return scbClient.Single(ctx, opts)
	}

	return scbClient.Get(ctx, guid)
}

// Create creates a ServiceCredentialBinding resource
func Create(ctx context.Context, scbClient ServiceCredentialBinding, cr v1alpha1.ServiceCredentialBinding, params json.RawMessage) (*resource.ServiceCredentialBinding, error) {
	cr.Status.AtProvider.Name = *randomName(*cr.Spec.ForProvider.Name)
	opt, err := newCreateOption(cr, params)
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

	opts, err := newListOptions(cr)
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
func newListOptions(cr v1alpha1.ServiceCredentialBinding) (*client.ServiceCredentialBindingListOptions, error) {
	// if external-name is not set, search by Name and Space
	opt := client.NewServiceCredentialBindingListOptions()
	opt.Type.EqualTo(cr.Spec.ForProvider.Type)

	if cr.Spec.ForProvider.ServiceInstance == nil {
		return nil, errors.New(ErrServiceInstanceMissing)
	}
	opt.ServiceInstanceGUIDs.EqualTo(*cr.Spec.ForProvider.ServiceInstance)

	if cr.Spec.ForProvider.Type == "app" {
		if cr.Spec.ForProvider.App == nil {
			return nil, errors.New(ErrAppMissing)
		}
		opt.AppGUIDs.EqualTo(*cr.Spec.ForProvider.App)
	}

	if cr.Spec.ForProvider.Type == "key" {
		if cr.Status.AtProvider.Name == "" {
			return nil, errors.New(ErrNameMissing)
		}
		opt.Names.EqualTo(cr.Status.AtProvider.Name)
	}

	return opt, nil
}

// newCreateOption generates ServiceCredentialBindingCreate according to CR's ForProvider spec
func newCreateOption(cr v1alpha1.ServiceCredentialBinding, params json.RawMessage) (*resource.ServiceCredentialBindingCreate, error) {
	if cr.Spec.ForProvider.ServiceInstance == nil {
		return nil, errors.New(ErrServiceInstanceMissing)
	}

	var opt *resource.ServiceCredentialBindingCreate
	switch cr.Spec.ForProvider.Type {
	case "key":
		if cr.Status.AtProvider.Name == "" {
			return nil, errors.New(ErrNameMissing)
		}

		opt = resource.NewServiceCredentialBindingCreateKey(*cr.Spec.ForProvider.ServiceInstance, cr.Status.AtProvider.Name)
	case "app":
		if cr.Spec.ForProvider.App == nil {
			return nil, errors.New(ErrAppMissing)
		}
		opt = resource.NewServiceCredentialBindingCreateApp(*cr.Spec.ForProvider.ServiceInstance, *cr.Spec.ForProvider.App)

		// for app binding, binding name is optional
		if cr.Spec.ForProvider.Name != nil {
			opt.WithName(*cr.Spec.ForProvider.Name)
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

func randomName(name string) *string {
	if len(name) > 0 && name[len(name)-1] == '-' {
		name = name[:len(name)-1]
	}
	newName := name + "-" + randomString(5)
	return &newName
}

const letterBytes = "abcdefghijklmnopqrstuvwxyz1234567890"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

var src = rand.NewSource(time.Now().UnixNano())

func randomString(n int) string {
	sb := strings.Builder{}
	sb.Grow(n)

	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			sb.WriteByte(letterBytes[idx])
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return sb.String()
}
