package servicecredentialbinding

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/google/uuid"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/job"
)

const (
	ErrServiceInstanceMissing = "service instance is required for key/app binding"
	ErrAppMissing             = "app is required for app binding"
	ErrNameMissing            = "name is required for key binding"
	ErrBindingTypeUnknown     = "unknown binding type. supported types are key and app"
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

	return scbClient.Single(ctx, createToListOptions(opt))
}

// Update updates labels and annotations of a ServiceCredentialBinding resource
func Update(ctx context.Context, scbClient ServiceCredentialBinding, guid string, forProvider v1alpha1.ServiceCredentialBindingParameters) (*resource.ServiceCredentialBinding, error) {
	opt := newUpdateOption(forProvider)
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
func newListOptions(forProvider v1alpha1.ServiceCredentialBindingParameters) (*client.ServiceCredentialBindingListOptions, error) {
	// if external-name is not set, search by Name and Space
	opt := client.NewServiceCredentialBindingListOptions()
	opt.Type.EqualTo(forProvider.Type)

	if forProvider.ServiceInstance == nil {
		return nil, errors.New(ErrServiceInstanceMissing)
	}
	opt.ServiceInstanceGUIDs.EqualTo(*forProvider.ServiceInstance)

	if forProvider.Type == "app" {
		if forProvider.App == nil {
			return nil, errors.New(ErrAppMissing)
		}
		opt.AppGUIDs.EqualTo(*forProvider.App)
	}

	if forProvider.Type == "key" {
		if forProvider.Name == nil {
			return nil, errors.New(ErrNameMissing)
		}
		opt.Names.EqualTo(*forProvider.Name)
	}

	return opt, nil
}

// newCreateOption generates ServiceCredentialBindingCreate according to CR's ForProvider spec
func newCreateOption(forProvider v1alpha1.ServiceCredentialBindingParameters, params json.RawMessage) (*resource.ServiceCredentialBindingCreate, error) {
	if forProvider.ServiceInstance == nil {
		return nil, errors.New(ErrServiceInstanceMissing)
	}

	var opt *resource.ServiceCredentialBindingCreate
	switch forProvider.Type {
	case "key":
		if forProvider.Name == nil {
			return nil, errors.New(ErrNameMissing)
		}

		name := randomName(*forProvider.Name)

		opt = resource.NewServiceCredentialBindingCreateKey(*forProvider.ServiceInstance, name)
	case "app":
		if forProvider.App == nil {
			return nil, errors.New(ErrAppMissing)
		}
		opt = resource.NewServiceCredentialBindingCreateApp(*forProvider.ServiceInstance, *forProvider.App)

		// for app binding, binding name is optional
		if forProvider.Name != nil {
			opt.WithName(*forProvider.Name)
		}
	default:
		return nil, errors.New(ErrBindingTypeUnknown)
	}

	if params != nil {
		opt.WithJSONParameters(string(params))
	}
	return opt, nil
}

func createToListOptions(create *resource.ServiceCredentialBindingCreate) *client.ServiceCredentialBindingListOptions {
	// create options are not used in the controller, but can be used in tests
	opts := client.NewServiceCredentialBindingListOptions()
	opts.Type.EqualTo(create.Type)

	opts.ServiceInstanceGUIDs.EqualTo(create.Relationships.ServiceInstance.Data.GUID)

	if create.Type == "app" && create.Relationships.App != nil {
		opts.AppGUIDs.EqualTo(create.Relationships.App.Data.GUID)
	}

	if create.Type == "key" && create.Name != nil {
		opts.Names.EqualTo(*create.Name)
	}

	return opts
}

// newUpdateOption generates ServiceCredentialBindingUpdate according to CR's ForProvider spec
func newUpdateOption(forProvider v1alpha1.ServiceCredentialBindingParameters) *resource.ServiceCredentialBindingUpdate {
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
func IsUpToDate(ctx context.Context, forProvider v1alpha1.ServiceCredentialBindingParameters, r resource.ServiceCredentialBinding) bool {
	// SCB support updates for labels and metadata only. This is to be implemented. For now return true
	return true
}

func randomName(name string) string {
	if len(name) > 0 && name[len(name)-1] == '-' {
		name = name[:len(name)-1]
	}
	newName := name + "-" + randomString(5)
	return newName
}

const letterBytes = "abcdefghijklmnopqrstuvwxyz1234567890"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

var (
	src      = rand.NewSource(time.Now().UnixNano())
	srcMutex sync.Mutex
)

func randomString(n int) string {
	sb := strings.Builder{}
	sb.Grow(n)

	srcMutex.Lock()
	defer srcMutex.Unlock()

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
