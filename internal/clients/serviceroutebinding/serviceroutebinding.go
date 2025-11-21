package serviceroutebinding

import (
	"context"
	"strings"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/uuid"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/job"
	cfclient "github.com/cloudfoundry/go-cfclient/v3/client"
	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
)

// using this client https://pkg.go.dev/github.com/cloudfoundry/go-cfclient/v3@v3.0.0-alpha.12/client#ServiceRouteBindingClient
const (
	resourceType                = "ServiceRouteBinding"
	externalSystem              = "Cloud Foundry"
	errTrackPCUsage             = "cannot track ProviderConfig usage: %w"
	errNewClient                = "cannot create a client for " + externalSystem + ": %w"
	errWrongCRType              = "managed resource is not a " + resourceType
	errGet                      = "cannot get " + resourceType + " in " + externalSystem + ": %w"
	errFind                     = "cannot find " + resourceType + " in " + externalSystem
	errCreate                   = "cannot create " + resourceType + " in " + externalSystem + ": %w"
	errUpdate                   = "cannot update " + resourceType + " in " + externalSystem + ": %w"
	errDelete                   = "cannot delete " + resourceType + " in " + externalSystem + ": %w"
	errUpdateStatus             = "cannot update status after retiring binding: %w"
	errExtractParams            = "cannot extract specified parameters: %w"
	errUnknownState             = "unknown last operation state for " + resourceType + " in " + externalSystem
	errMissingRelationshipGUIDs = "missing relationship GUIDs (route=%q serviceInstance=%q)"
	errServiceInstanceNotFound  = "referenced ServiceInstance %q not found"
)

type serviceRouteBinding interface {
	Get(ctx context.Context, guid string) (*resource.ServiceRouteBinding, error)
	Single(ctx context.Context, opts *client.ServiceRouteBindingListOptions) (*resource.ServiceRouteBinding, error)
	Create(ctx context.Context, r *resource.ServiceRouteBindingCreate) (string, *resource.ServiceRouteBinding, error)
	Update(ctx context.Context, guid string, r *resource.ServiceRouteBindingUpdate) (*resource.ServiceRouteBinding, error)
	Delete(context.Context, string) (string, error)
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

func GetByIDOrSearch(ctx context.Context, srbClient ServiceRouteBinding, guid string, forProvider v1alpha1.ServiceRouteBindingParameters) (*cfresource.ServiceRouteBinding, error) {

	if err := uuid.Validate(guid); err == nil {
		// try to find by GUID
		return srbClient.Get(ctx, guid)
	} else {
		// search by spec
		opts := cfclient.NewServiceRouteBindingListOptions()
		opts.RouteGUIDs.EqualTo(forProvider.RouteGUID)
		opts.ServiceInstanceGUIDs.EqualTo(forProvider.ServiceInstanceGUID)
		return srbClient.Single(ctx, opts)
	}
}

func Create(ctx context.Context, srbClient ServiceRouteBinding, forProvider v1alpha1.ServiceRouteBindingParameters) (*resource.ServiceRouteBinding, error) {
	opt, err := newCreateOption(forProvider)
	if err != nil {
		return nil, err
	}
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

func newCreateOption(forProvider v1alpha1.ServiceRouteBindingParameters) (*cfresource.ServiceRouteBindingCreate, error) {
	// need to be implimented depending on selected structure
	// -------------------------------------------------------------------------------------------------
	// Link modeling options:
	// We need a required 'self' link plus any number of additional dynamic links returned by CF (e.g. service_instance, route, parameters).
	// Option 1 uses a flat map (LinksMap) matching CF JSON exactly, but cannot enforce 'self' at schema level (must validate in controller).
	// Option 2 (active) uses a struct with a required Self field and an 'additional' map to hold any other links, enabling schema enforcement.
	// TODO: find out if its just service_instance, route, parameters fields (Typed) or dynamic keys!!!
	// check out proposed solution https://github.com/SAP/crossplane-provider-cloudfoundry/issues/81
	return cfresource.NewServiceRouteBindingCreate(forProvider.RouteGUID, forProvider.ServiceInstanceGUID), nil
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

func UpdateObservation(observation *v1alpha1.ServiceRouteBindingObservation, r *resource.ServiceRouteBinding) {
	observation.GUID = r.GUID
	if !r.CreatedAt.IsZero() {
		formatted := r.CreatedAt.UTC().Format(time.RFC3339)
		observation.CreatedAt = &formatted
	}
	observation.LastOperation = &v1alpha1.LastOperation{
		Type:      r.LastOperation.Type,
		State:     r.LastOperation.State,
		CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: r.CreatedAt.UTC().Format(time.RFC3339),
	}
	observation.RouteServiceUrl = r.RouteServiceURL
	/*
		observation.Relationships = v1alpha1.Relation{
			Route:           v1alpha1.Data{GUID: r.Relationships.Route.Data.GUID},
			ServiceInstance: v1alpha1.Data{GUID: r.Relationships.ServiceInstance.Data.GUID},
		}
	*/
	observation.Links = v1alpha1.Links{
		Self:       buildSelfLink(r.Links),
		Additional: buildAdditionalLinks(r.Links),
	}
	observation.ResourceMetadata = v1alpha1.ResourceMetadata{
		Labels:      r.Metadata.Labels,
		Annotations: r.Metadata.Annotations,
	}
}

// returns the 'self' link from CF links
func buildSelfLink(cfLinks cfresource.Links) v1alpha1.Link {
	var self v1alpha1.Link
	if cfLinks == nil {
		return self
	}
	cfSelf := cfLinks.Self()
	self.Href = cfSelf.Href
	if cfSelf.Method != "" {
		self.Method = &cfSelf.Method
	}
	return self
}

// builds additional links map from CF links excluding 'self'
func buildAdditionalLinks(cfLinks cfresource.Links) map[string]v1alpha1.Link {
	if cfLinks == nil {
		return nil
	}
	var additional map[string]v1alpha1.Link
	for k, v := range cfLinks {
		if strings.EqualFold(k, "self") {
			continue
		}
		l := v1alpha1.Link{Href: v.Href}
		if v.Method != "" {
			l.Method = &v.Method
		}
		if additional == nil {
			additional = make(map[string]v1alpha1.Link)
		}
		additional[k] = l
	}
	return additional
}

/*
func getServiceInstance(ctx context.Context, kube k8s.Client, cr *v1alpha1.ServiceRouteBinding) (*v1alpha1.ServiceInstance, error) {
	si := &v1alpha1.ServiceInstance{}
	if err := kube.Get(ctx, k8s.ObjectKey{
		Namespace: cr.Spec.ForProvider.ServiceInstanceRef.Namespace,
		Name:      cr.Spec.ForProvider.ServiceInstanceRef.Name,
	}, si); err != nil {
		return nil, err
	}
	return si, nil
}

func setServiceInstance(ctx context.Context, kube k8s.Client, cr *v1alpha1.ServiceRouteBinding) error {
	si, err := getServiceInstance(ctx, kube, cr)
	if err != nil {
		return err
	}
	cr.Status.AtProvider.ServiceInstanceGUID = *si.Status.AtProvider.ID
	cr.Status.AtProvider.RouteServiceUrl = *si.Status.AtProvider.RouteServiceURL
	return nil
}

func getRoute(ctx context.Context, kube k8s.Client, cr *v1alpha1.ServiceRouteBinding) (*v1alpha1.Route, error) {
	route := &v1alpha1.Route{}
	if err := kube.Get(ctx, k8s.ObjectKey{
		Namespace: cr.Spec.ForProvider.RouteRef.Namespace,
		Name:      cr.Spec.ForProvider.RouteRef.Name,
	}, route); err != nil {
		return nil, err
	}
	return route, nil
}

func setRoute(ctx context.Context, kube k8s.Client, cr *v1alpha1.ServiceRouteBinding) error {
	route, err := getRoute(ctx, kube, cr)
	if err != nil {
		return err
	}
	cr.Status.AtProvider.RouteGUID = route.Status.AtProvider.GUID
	return nil
}
*/
