package route

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
)

// Route is the interface that defines the methods that a Route client should implement.
type Route interface {
	Get(ctx context.Context, guid string) (*resource.Route, error)
	Single(ctx context.Context, opts *client.RouteListOptions) (*resource.Route, error)
	Create(ctx context.Context, r *resource.RouteCreate) (*resource.Route, error)
	Update(ctx context.Context, guid string, r *resource.RouteUpdate) (*resource.Route, error)
	Delete(ctx context.Context, guid string) (string, error)
}

type Client struct {
	Route
}

// NewClient creates a new cf client and return interfaces for Route and RouteFeatures
func NewClient(cf *client.Client) *Client {
	return &Client{
		Route: cf.Routes,
	}
}

// GetByIDOrSpec returns a Route by ID or by forProvider Spec.
func (c *Client) GetByIDOrSpec(ctx context.Context, guid string, forProvider v1alpha1.RouteParameters) (*v1alpha1.RouteObservation, error) {
	var r *resource.Route
	var err error
	if clients.IsValidGUID(guid) {
		r, err = c.Route.Get(ctx, guid)
	} else {
		opts, e := FormatListOption(forProvider)
		if e != nil {
			return nil, e
		}
		r, err = c.Route.Single(ctx, opts)
	}
	if err != nil {
		if clients.ErrorIsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	atProvider := GenerateObservation(r)
	return &atProvider, nil

}

// Create creates a Route and returns the GUID or error
func (c *Client) Create(ctx context.Context, forProvider v1alpha1.RouteParameters) (string, error) {
	opts, err := FormatCreateOption(forProvider)
	if err != nil {
		return "", err
	}

	r, err := c.Route.Create(ctx, opts)
	if err != nil {
		return "", err
	}
	return r.GUID, nil
}

// Update updates a Route
func (c *Client) Update(ctx context.Context, guid string, forProvider v1alpha1.RouteParameters) error {
	if !clients.IsValidGUID(guid) {
		return fmt.Errorf("invalid Route GUID")
	}

	opts := FormatUpdateOption(forProvider)
	if opts == nil {
		return fmt.Errorf("invalid Route parameters")
	}

	_, err := c.Route.Update(ctx, guid, opts)
	return err
}

func (c *Client) Delete(ctx context.Context, guid string) error {
	if !clients.IsValidGUID(guid) {
		return fmt.Errorf("invalid Route GUID")
	}

	_, err := c.Route.Delete(ctx, guid)
	if err != nil {
		return err
	}
	return nil
}

// FormatListOption generates the list options for the client.
func FormatListOption(forProvider v1alpha1.RouteParameters) (*client.RouteListOptions, error) {

	if forProvider.Space == nil || forProvider.Domain == nil {
		return nil, fmt.Errorf("Space and Domain are required")
	}
	opts := client.NewRouteListOptions()
	opts.SpaceGUIDs = client.Filter{Values: []string{*forProvider.Space}}
	opts.DomainGUIDs = client.Filter{Values: []string{*forProvider.Domain}}

	if forProvider.Host != nil {
		opts.Hosts = client.Filter{Values: []string{*forProvider.Host}}
	}

	if forProvider.Path != nil {
		opts.Paths = client.Filter{Values: []string{*forProvider.Path}}
	}

	if forProvider.Port != nil {
		opts.Ports = client.Filter{Values: []string{fmt.Sprintf("%d", *forProvider.Port)}}
	}

	return opts, nil
}

// FormatCreateOption generates the RouteCreate from the forProvider spec
func FormatCreateOption(forProvider v1alpha1.RouteParameters) (*resource.RouteCreate, error) {
	if forProvider.Space == nil || forProvider.Domain == nil {
		return nil, fmt.Errorf("Space and Domain are required")
	}

	opts := resource.NewRouteCreate(*forProvider.Domain, *forProvider.Space)

	if forProvider.Host != nil {
		opts.Host = forProvider.Host
	}

	if forProvider.Path != nil {
		opts.Path = forProvider.Path
	}

	if forProvider.Port != nil {
		opts.Port = forProvider.Port
	}

	return opts, nil
}

// FormatUpdateOption generates the RouteCreate from an *RouteParameters
func FormatUpdateOption(forProvider v1alpha1.RouteParameters) *resource.RouteUpdate {
	// client supports only updating metadata
	return &resource.RouteUpdate{
		Metadata: &resource.Metadata{},
	}
}

// GenerateObservation takes an Route resource and returns *RouteObservation.
func GenerateObservation(o *resource.Route) v1alpha1.RouteObservation {
	resource := v1alpha1.Resource{
		GUID:      o.GUID,
		CreatedAt: strToPtr(o.CreatedAt.Format(time.RFC3339)),
		UpdatedAt: strToPtr(o.UpdatedAt.Format(time.RFC3339)),
	}
	obs := v1alpha1.RouteObservation{Resource: resource}

	obs.URL = strToPtr(o.URL)
	obs.Host = strToPtr(o.Host)
	obs.Path = strToPtr(o.Path)
	obs.Protocol = strToPtr(o.Protocol)

	if o.Destinations != nil {
		obs.Destinations = make([]v1alpha1.RouteDestination, 0, len(o.Destinations))
		for _, d := range o.Destinations {
			if d.GUID == nil {
				continue
			}

			rd := v1alpha1.RouteDestination{
				GUID: *d.GUID,
			}
			if d.Port != nil {
				rd.Port = d.Port
			}

			if d.App.GUID != nil {
				rd.App = &v1alpha1.RouteDestinationApp{GUID: *d.App.GUID}
				if d.App.Process != nil {
					proc := *d.App.Process
					rd.App.Process = strToPtr(proc.Type)
				}
			}

			obs.Destinations = append(obs.Destinations, rd)

		}
	}
	return obs
}

// IsUpToDate checks whether current state is up-to-date compared to the given
// set of parameters.
func IsUpToDate(forProvider v1alpha1.RouteParameters, atProvider v1alpha1.RouteObservation) bool {
	// Routes are mostly immutable, expect for metadata
	return true
}

func strToPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
