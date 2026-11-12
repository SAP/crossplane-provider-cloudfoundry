package servicecredentialbinding

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	xpresource "github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/google/uuid"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/job"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/metadata"
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
	GetParameters(ctx context.Context, guid string) (*json.RawMessage, error)
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
func Create(ctx context.Context, scbClient ServiceCredentialBinding, mg xpresource.Managed, forProvider v1alpha1.ServiceCredentialBindingParameters, params json.RawMessage) (*resource.ServiceCredentialBinding, error) {
	opt, err := newCreateOption(mg, forProvider, params)
	if err != nil {
		return nil, err
	}

	// usually the binding is not ready yet at this point and is empty
	jobGUID, binding, err := scbClient.Create(ctx, opt)
	if err != nil {
		return binding, err
	}

	if jobGUID != "" { // async creation waits for the job to complete
		if pollErr := job.PollJobComplete(ctx, scbClient, jobGUID); pollErr != nil {
			// Capture the external name even on error - otherwise a failed create
			// is invisible to Observe(), which creates another binding instead of
			// recognizing the existing one, repeating until the circuit breaker
			// trips and quota may be exhausted.
			if found, lookupErr := scbClient.Single(ctx, createToListOptions(opt)); lookupErr == nil {
				return found, pollErr
			}
			return nil, pollErr
		}
	}

	return scbClient.Single(ctx, createToListOptions(opt))
}

// Update updates labels and annotations of a ServiceCredentialBinding resource
func Update(ctx context.Context, scbClient ServiceCredentialBinding, guid string, mg xpresource.Managed, forProvider v1alpha1.ServiceCredentialBindingParameters) (*resource.ServiceCredentialBinding, error) {
	opt := newUpdateOption(mg, forProvider)
	return scbClient.Update(ctx, guid, opt)
}

// ErrDeletionPending means Cloud Foundry still reports the binding or its delete operation.
var ErrDeletionPending = errors.New("service credential binding deletion is pending")

// Delete deletes a ServiceCredentialBinding resource and confirms its absence.
// A successful DELETE response may only mean that Cloud Foundry accepted an
// asynchronous unbind operation, so the binding identity must remain tracked
// until a GET confirms it no longer exists.
func Delete(ctx context.Context, scbClient ServiceCredentialBinding, guid string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cannot confirm deletion of service credential binding %s: %w", guid, err)
	}

	binding, gone, err := getBindingForDeletion(ctx, scbClient, guid, "before deletion")
	if err != nil {
		return err
	}
	if gone {
		return nil
	}
	if err := pendingDeleteOperation(binding, guid); err != nil {
		return err
	}

	gone, err = deleteBinding(ctx, scbClient, guid)
	if err != nil {
		return err
	}
	if gone {
		return nil
	}
	gone, err = confirmBindingDeletion(ctx, scbClient, guid)
	if err != nil {
		return err
	}
	if gone {
		return nil
	}
	return fmt.Errorf("%w: Cloud Foundry still returns binding %s", ErrDeletionPending, guid)
}

func confirmBindingDeletion(ctx context.Context, scbClient ServiceCredentialBinding, guid string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, fmt.Errorf("cannot confirm deletion of service credential binding %s: %w", guid, err)
	}
	_, gone, err := getBindingForDeletion(ctx, scbClient, guid, "while confirming deletion")
	return gone, err
}

func deleteBinding(ctx context.Context, scbClient ServiceCredentialBinding, guid string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, fmt.Errorf("cannot delete service credential binding %s: %w", guid, err)
	}
	if _, err := scbClient.Delete(ctx, guid); err != nil {
		if isBindingNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("cannot delete service credential binding %s: %w", guid, err)
	}
	return false, nil
}

func getBindingForDeletion(ctx context.Context, scbClient ServiceCredentialBinding, guid, phase string) (*resource.ServiceCredentialBinding, bool, error) {
	binding, err := scbClient.Get(ctx, guid)
	if isBindingNotFound(err) {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("cannot check service credential binding %s %s: %w", guid, phase, err)
	}
	if binding == nil {
		return nil, false, fmt.Errorf("%w: Cloud Foundry returned no binding %s for %s", ErrDeletionPending, phase, guid)
	}
	return binding, false, nil
}

func pendingDeleteOperation(binding *resource.ServiceCredentialBinding, guid string) error {
	if binding.LastOperation.Type != v1alpha1.LastOperationDelete {
		return nil
	}
	switch binding.LastOperation.State {
	case v1alpha1.LastOperationInitial, v1alpha1.LastOperationInProgress, v1alpha1.LastOperationSucceeded:
		return fmt.Errorf("%w: Cloud Foundry delete operation for %s is %s", ErrDeletionPending, guid, binding.LastOperation.State)
	case v1alpha1.LastOperationFailed:
		// Retry a broker-failed unbind below.
	default:
		return fmt.Errorf("%w: Cloud Foundry delete operation for %s has unknown state %q", ErrDeletionPending, guid, binding.LastOperation.State)
	}
	return nil
}

func isBindingNotFound(err error) bool {
	return resource.IsResourceNotFoundError(err) || resource.IsServiceBindingNotFoundError(err)
}

// GetConnectionDetails returns the connection details of the ServiceCredentialBinding.
// A nil result with an error indicates failed retrieval or encoding; an empty, non-nil
// result with no error is a successful response containing no credentials.
func GetConnectionDetails(ctx context.Context, scbClient ServiceCredentialBinding, guid string, asJSON bool) (managed.ConnectionDetails, error) {
	bindingDetails, err := scbClient.GetDetails(ctx, guid)
	if err != nil {
		return nil, fmt.Errorf("cannot retrieve service binding credentials: %w", err)
	}
	if bindingDetails == nil {
		return nil, errors.New("cannot retrieve service binding credentials: Cloud Foundry returned no details")
	}

	jsonCredentials, err := json.Marshal(bindingDetails.Credentials)
	if err != nil {
		return nil, errors.New("cannot encode service binding credentials")
	}

	connectDetails := managed.ConnectionDetails{}
	if asJSON {
		connectDetails["credentials"] = jsonCredentials
		return connectDetails, nil
	}

	for key, value := range normalizeMap(bindingDetails.Credentials, make(map[string]string), "", "_") {
		connectDetails[key] = []byte(value)
	}

	return connectDetails, nil
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
func newCreateOption(mg xpresource.Managed, forProvider v1alpha1.ServiceCredentialBindingParameters, params json.RawMessage) (*resource.ServiceCredentialBindingCreate, error) {
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
	opt.Metadata = metadata.BuildMetadata(mg, forProvider.Labels, forProvider.Annotations)
	return opt, nil
}

func createToListOptions(create *resource.ServiceCredentialBindingCreate) *client.ServiceCredentialBindingListOptions {
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
func newUpdateOption(mg xpresource.Managed, forProvider v1alpha1.ServiceCredentialBindingParameters) *resource.ServiceCredentialBindingUpdate {
	opt := &resource.ServiceCredentialBindingUpdate{}
	opt.Metadata = metadata.BuildMetadata(mg, forProvider.Labels, forProvider.Annotations)
	return opt
}

// UpdateObservation updates the CR's AtProvider status from the observed resource
func UpdateObservation(observation *v1alpha1.ServiceCredentialBindingObservation, r *resource.ServiceCredentialBinding) {
	observation.GUID = r.GUID
	observation.Name = ptr.Deref(r.Name, "")
	observation.LastOperation = &v1alpha1.LastOperation{
		Type:        r.LastOperation.Type,
		State:       r.LastOperation.State,
		Description: r.LastOperation.Description,
		UpdatedAt:   r.LastOperation.UpdatedAt.String(),
		CreatedAt:   r.LastOperation.CreatedAt.String(),
	}

	if r.Metadata != nil {
		observation.Labels = r.Metadata.Labels
		observation.Annotations = r.Metadata.Annotations
	}
}

// IsUpToDate checks whether the CR is up to date with the observed resource
func IsUpToDate(ctx context.Context, mg xpresource.Managed, forProvider v1alpha1.ServiceCredentialBindingParameters, r resource.ServiceCredentialBinding) bool {
	// SCB supports updates for labels and annotations only
	desired := metadata.BuildMetadata(mg, forProvider.Labels, forProvider.Annotations)
	var actualLabels, actualAnnotations map[string]*string
	if r.Metadata != nil {
		actualLabels = r.Metadata.Labels
		actualAnnotations = r.Metadata.Annotations
	}
	return metadata.IsMetadataUpToDate(desired.Labels, desired.Annotations, actualLabels, actualAnnotations)
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
