package serviceinstance

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
)

// ServiceInstance defines interfaces to the ServiceInstance resource
type ServiceInstance interface {
	Get(context.Context, string) (*resource.ServiceInstance, error)
	GetManagedParameters(context.Context, string) (*json.RawMessage, error)
	GetUserProvidedCredentials(context.Context, string) (*json.RawMessage, error)
	Single(context.Context, *client.ServiceInstanceListOptions) (*resource.ServiceInstance, error)
	CreateManaged(context.Context, *resource.ServiceInstanceManagedCreate) (string, error)
	UpdateManaged(context.Context, string, *resource.ServiceInstanceManagedUpdate) (string, *resource.ServiceInstance, error)
	CreateUserProvided(context.Context, *resource.ServiceInstanceUserProvidedCreate) (*resource.ServiceInstance, error)
	UpdateUserProvided(context.Context, string, *resource.ServiceInstanceUserProvidedUpdate) (*resource.ServiceInstance, error)
	Delete(context.Context, string) (string, error)
}

// Job defines interfaces to async operations/jobs.
type Job interface {
	PollComplete(context.Context, string, *client.PollingOptions) error
}

// newPollingOptions creates a new polling options with a timeout
var pollInterval = time.Second * 10
var pollTimeout = time.Minute * 1 // this can be shorter than creation time because we have logic to observe async operation state

func newPollingOptions() *client.PollingOptions {
	p := client.NewPollingOptions()
	p.Timeout = pollTimeout
	p.CheckInterval = pollInterval
	return p
}

// PollJobComplete polls for completion with extended timeout
func (c *Client) pollJobComplete(ctx context.Context, job string) error {
	ctx, cancel := context.WithTimeout(ctx, pollTimeout)
	defer cancel()

	err := c.Job.PollComplete(ctx, job, newPollingOptions())

	if err != nil && errors.Is(err, client.AsyncProcessTimeoutError) { // because we have logic to observe job state, we can safely ignore timeout error
		return nil
	}
	return err
}

// Client operates on ServiceInstance resources and uses Job to poll async operations.
type Client struct {
	ServiceInstance
	Job
}

// NewClient creates a new client instance from a cfclient.ServiceInstance instance.
func NewClient(cf *client.Client) *Client {
	return &Client{cf.ServiceInstances, cf.Jobs}
}

// GetByIDOrSpec retrieves external resource by GUID or by matching CR's ForProvider spec
func GetByIDOrSpec(ctx context.Context, c *Client, guid string, spec v1alpha1.ServiceInstanceParameters) (*resource.ServiceInstance, error) {
	if _, err := uuid.Parse(guid); err == nil {
		return c.Get(ctx, guid)
	}

	return c.MatchSingle(ctx, spec)
}

// Get retrieves external resource using GUID
func (c *Client) Get(ctx context.Context, guid string) (*resource.ServiceInstance, error) {
	if guid == "" {
		return nil, nil
	}
	return c.ServiceInstance.Get(ctx, guid)
}

// MatchSingle retrieves external resource by matching CR's ForProvider spec
func (c *Client) MatchSingle(ctx context.Context, spec v1alpha1.ServiceInstanceParameters) (*resource.ServiceInstance, error) {
	// if external-name is not set, search by Name and Space
	opt := client.NewServiceInstanceListOptions()
	opt.Type = string(spec.Type)
	opt.Names.EqualTo(*spec.Name)
	if spec.Space != nil && *spec.Space != "" {
		opt.SpaceGUIDs.EqualTo(*spec.Space)
	}

	if spec.ServicePlan != nil && *spec.ServicePlan.ID != "" {
		opt.ServicePlanGUIDs.EqualTo(*spec.ServicePlan.ID)
	}

	// Use Single as exact one match is possible in a Cloud Foundry Space
	r, err := c.ServiceInstance.Single(ctx, opt)

	if err == nil {
		return r, nil
	}

	// Ignore errors if no results or exactly one result is not returned
	if errors.Is(err, client.ErrExactlyOneResultNotReturned) || errors.Is(err, client.ErrNoResultsReturned) {
		return nil, nil
	}

	return nil, err
}

// GetServiceCredentials retrieves service instance credentials
func (c *Client) GetServiceCredentials(ctx context.Context, r *resource.ServiceInstance) (json.RawMessage, error) {
	if r == nil {
		return nil, nil
	}

	if r.Type == string(v1alpha1.ManagedService) {
		raw, err := c.ServiceInstance.GetManagedParameters(ctx, r.GUID)
		if raw == nil {
			return nil, err
		}
		return *raw, err
	}

	raw, err := c.ServiceInstance.GetUserProvidedCredentials(ctx, r.GUID)
	if raw == nil {
		return nil, err
	}
	return *raw, err
}

// Create creates the external resource according to CR's ForProvider spec
func (c *Client) Create(ctx context.Context, spec v1alpha1.ServiceInstanceParameters, creds json.RawMessage) (*resource.ServiceInstance, error) {
	switch spec.Type {
	case v1alpha1.ManagedService:
		return c.createManaged(ctx, spec, creds)

	case v1alpha1.UserProvidedService:
		return c.createUserProvided(ctx, spec, creds)
	default:
		return nil, errors.New("unknown service instance type")
	}
}

// createManaged creates a managed service instance according to CR's ForProvider spec
func (c *Client) createManaged(ctx context.Context, spec v1alpha1.ServiceInstanceParameters, params json.RawMessage) (*resource.ServiceInstance, error) {

	// throw error if no space is provided
	if spec.Space == nil {
		return nil, errors.New("no space reference provided")
	}

	opt := resource.NewServiceInstanceCreateManaged(*spec.Name, *spec.Space, *spec.ServicePlan.ID)

	if params != nil {
		opt.Parameters = &params
	}

	job, err := c.ServiceInstance.CreateManaged(ctx, opt)
	if err != nil {
		return nil, err
	}
	// Poll for completion
	if err = c.pollJobComplete(ctx, job); err != nil {
		return nil, err
	}

	return c.MatchSingle(ctx, spec)
}

// createUserProvided creates a user-provided service instance according to CR's ForProvider spec
func (c *Client) createUserProvided(ctx context.Context, spec v1alpha1.ServiceInstanceParameters, creds json.RawMessage) (*resource.ServiceInstance, error) {
	// Credential is required for UPS
	if creds == nil {
		return nil, errors.New("Missing or invalid credentials")
	}

	// throw error if no space is provided
	if spec.Space == nil {
		return nil, errors.New("no space reference provided")
	}
	// create the service instance
	opt := resource.NewServiceInstanceCreateUserProvided(*spec.Name, *spec.Space)
	si, err := c.ServiceInstance.CreateUserProvided(ctx, opt)
	if err != nil {
		return nil, err
	}

	// workaround: cf-goclient supports few ups options at creation time.
	upt := resource.NewServiceInstanceUserProvidedUpdate().
		WithCredentials(creds).
		WithRouteServiceURL(spec.RouteServiceURL).
		WithSyslogDrainURL(spec.SyslogDrainURL)

	return c.ServiceInstance.UpdateUserProvided(ctx, si.GUID, upt)
}

// Update updates the external resource to keep it in sync with CR's ForProvider spec
func (c *Client) Update(ctx context.Context, guid string, desired *v1alpha1.ServiceInstanceParameters, creds json.RawMessage) (*resource.ServiceInstance, error) {
	observed, err := c.Get(ctx, guid)
	if err != nil {
		return nil, err
	}
	switch desired.Type {
	case v1alpha1.ManagedService:
		return c.updateManaged(ctx, observed, desired, creds)
	case v1alpha1.UserProvidedService:
		return c.updateUserProvided(ctx, observed, desired, creds)
	default:
		return nil, errors.New("unknown service instance type")
	}
}

// updateManaged updates managed service instance according to CR's ForProvider spec
func (c *Client) updateManaged(ctx context.Context, observed *resource.ServiceInstance, desired *v1alpha1.ServiceInstanceParameters, params json.RawMessage) (*resource.ServiceInstance, error) {
	upd := resource.NewServiceInstanceManagedUpdate()

	if observed.Name != *desired.Name {
		upd.WithName(*desired.Name)
	}

	if desired.ServicePlan.ID != nil && observed.Relationships.ServicePlan.Data.GUID != *desired.ServicePlan.ID {
		upd.WithServicePlan(*desired.ServicePlan.ID)
	}

	if params != nil {
		upd.WithParameters(params)
	}

	// Update the service instance
	job, s, err := c.ServiceInstance.UpdateManaged(ctx, observed.GUID, upd)
	if err != nil {
		return nil, err
	}
	if job == "" {
		return s, nil
	}

	// Poll for completion
	if err = c.pollJobComplete(ctx, job); err != nil {
		return nil, err
	}

	return c.Get(ctx, observed.GUID)

}

// updateUserProvided updates user-provided service instance according to CR's ForProvider spec
func (c *Client) updateUserProvided(ctx context.Context, observed *resource.ServiceInstance, desired *v1alpha1.ServiceInstanceParameters, creds json.RawMessage) (*resource.ServiceInstance, error) {
	upd := resource.NewServiceInstanceUserProvidedUpdate()

	if creds == nil {
		return nil, errors.New("Missing or invalid credentials")

	}
	if observed.Name != *desired.Name {
		upd.WithName(*desired.Name)
	}

	upd.WithRouteServiceURL(desired.RouteServiceURL).WithSyslogDrainURL(desired.SyslogDrainURL).WithCredentials(creds)

	return c.ServiceInstance.UpdateUserProvided(ctx, observed.GUID, upd)
}

// Delete deletes a service instance managed by the CR
func (c *Client) Delete(ctx context.Context, cr *v1alpha1.ServiceInstance) error {
	job, err := c.ServiceInstance.Delete(ctx, *cr.Status.AtProvider.ID)

	// If the service instance is already deleted, return nil
	if clients.ErrorIsNotFound(err) {
		return nil
	}

	if err != nil {
		return err
	}

	// Poll for completion
	return c.pollJobComplete(ctx, job)
}

// LateInitialize populates EMPTY parameters based on the observed managed resource properties
func LateInitialize(p *v1alpha1.ServicePlanParameters, r *resource.ServiceInstance) {
	// nothing to do here
}

// UpdateObservation updates CR status based on the observed managed resource status
func UpdateObservation(in *v1alpha1.ServiceInstanceObservation, r *resource.ServiceInstance) {
	if r == nil {
		return
	}

	in.ID = &r.GUID
	in.LastOperation = v1alpha1.LastOperation{
		Type:        r.LastOperation.Type,
		State:       r.LastOperation.State,
		Description: r.LastOperation.Description,
		UpdatedAt:   r.LastOperation.UpdatedAt.String(),
	}

	if r.Type == string(v1alpha1.ManagedService) {
		in.ServicePlan = &r.Relationships.ServicePlan.Data.GUID
	}
}

// IsUpToDate checks if the managed resource is in sync with CR.
func IsUpToDate(in *v1alpha1.ServiceInstanceParameters, observed *resource.ServiceInstance) bool {
	if in.Name != nil && *in.Name != observed.Name {
		return false
	}

	switch in.Type {
	case v1alpha1.ManagedService:
		if in.ServicePlan != nil && in.ServicePlan.ID != nil && observed.Relationships.ServicePlan.Data.GUID != *in.ServicePlan.ID {
			return false
		}
	case v1alpha1.UserProvidedService:
		if in.RouteServiceURL != ptr.Deref(observed.RouteServiceURL, "") {
			return false
		}
		if in.SyslogDrainURL != ptr.Deref(observed.SyslogDrainURL, "") {
			return false
		}
	}
	return true
}
