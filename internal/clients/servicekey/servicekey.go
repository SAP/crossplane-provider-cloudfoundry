package servicekey

import (
	"context"
	"encoding/json"

	"github.com/cloudfoundry-community/go-cfclient/v3/client"
	"github.com/cloudfoundry-community/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"

	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/servicekey/v1alpha1"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/clients/cfclient"
)

// ServiceKey defines interfaces to the ServiceKey resource
type ServiceKey interface {
	Get(context.Context, string) (*resource.ServiceCredentialBinding, error)
	GetDetails(context.Context, string) (*resource.ServiceCredentialBindingDetails, error)
	Single(context.Context, *client.ServiceCredentialBindingListOptions) (*resource.ServiceCredentialBinding, error)
	Create(context.Context, *resource.ServiceCredentialBindingCreate) (string, *resource.ServiceCredentialBinding, error)
	Delete(context.Context, string) error
}

// Job defines interfaces to async operations/jobs.
type Job interface {
	PollComplete(context.Context, string, *client.PollingOptions) error
}

// Client uses ServiceKey to operate on ServiceKey resources and Job to poll async operations.
type Client struct {
	ServiceKey
	Job
}

// NewClient creates a new client instance from a cfclient.ServiceKey instance.
func NewClient(cf *cfclient.Client) *Client {
	return &Client{cf.ServiceCredentialBindings, cf.Jobs}
}

// Get retrieves external resource using CR's external_name (guid)
func (c *Client) Get(ctx context.Context, guid string) (*resource.ServiceCredentialBinding, error) {
	if guid == "" {
		return nil, nil
	}
	return c.ServiceKey.Get(ctx, guid)
}

// MatchSingle retrieves external resource using CR's spec
func (c *Client) MatchSingle(ctx context.Context, spec v1alpha1.ServiceKeyParameters) (*resource.ServiceCredentialBinding, error) {
	lo := client.NewServiceCredentialBindingListOptions()
	lo.ServiceInstanceGUIDs.EqualTo(*spec.ServiceInstance)
	lo.Names.EqualTo(*spec.Name)

	return c.ServiceKey.Single(ctx, lo)
}

// Create creates a managed service instance according to CR's ForProvider spec
func (c *Client) Create(ctx context.Context, spec v1alpha1.ServiceKeyParameters, param json.RawMessage) (*resource.ServiceCredentialBinding, error) {
	opt := resource.NewServiceCredentialBindingCreateKey(*spec.ServiceInstance, *spec.Name)

	// ignore json param in case of marshal error.
	if param != nil {
		p, err := json.Marshal(param)
		if err == nil {
			opt = opt.WithJSONParameters(string(p))
		}
	}

	job, binding, err := c.ServiceKey.Create(ctx, opt)
	if err != nil {
		return nil, err
	}

	if job != "" { // async creation
		if err := c.Job.PollComplete(ctx, job, client.NewPollingOptions()); err != nil {
			return nil, err
		}

		lo := client.NewServiceCredentialBindingListOptions()
		lo.ServiceInstanceGUIDs.EqualTo(*spec.ServiceInstance)
		lo.Names.EqualTo(*spec.Name)

		return c.ServiceKey.Single(ctx, lo)
	}

	return binding, nil
}

// LateInitialize populates EMPTY parameters based on the observed managed resource properties
func LateInitialize(p *v1alpha1.ServiceKeyParameters, r *resource.ServiceCredentialBinding) {
	// Nothing to do
}

// GenerateObservation updates CR status based on the observed managed resource status
func GenerateObservation(r *resource.ServiceCredentialBinding) v1alpha1.ServiceKeyObservation {
	// For now, just record the GUID of the service key.
	// TODO: Check requirement to see if it could be beneficial to record app that binds the key.
	o := v1alpha1.ServiceKeyObservation{}
	o.ID = &r.GUID
	return o
}

// IsUpToDate checks if the managed resource is in sync with CR.
func IsUpToDate(p *v1alpha1.ServiceKeyParameters, r *resource.ServiceCredentialBinding) bool {
	// Always, since none of the `ForProvider` parameters are updatable. Updatable are only  metadata and annotations.
	return true
}

// GetConnectionDetails extracts managed.ConnectionDetails out of ServiceKey
func (c *Client) GetConnectionDetails(ctx context.Context, guid string, asJSON bool) managed.ConnectionDetails {
	b, err := c.ServiceKey.GetDetails(ctx, guid)
	if err != nil {
		return managed.ConnectionDetails{}
	}

	if asJSON {
		buf, err := json.Marshal(b.Credentials)
		if err != nil {
			return managed.ConnectionDetails{}
		}

		return managed.ConnectionDetails{
			"credentials": buf,
		}
	}

	m := normalizeMap(b.Credentials, make(map[string]string), "", "_")

	mbyte := map[string][]byte{}

	for k, v := range m {
		mbyte[k] = []byte(v)
	}

	return mbyte
}
