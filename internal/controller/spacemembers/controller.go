package spacemembers

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudfoundry/go-cfclient/v3/config"
	"github.com/pkg/errors"

	ctrl "sigs.k8s.io/controller-runtime"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	apisv1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1alpha1"
	apisv1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/members"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/features"
)

const (
	errWrongKind         = "Managed resource is not an SpaceMembers kind"
	errTrackUsage        = "cannot track usage"
	errGetProviderConfig = "cannot get ProviderConfig or resolve credential references"
	errGetClient         = "cannot create a client to talk to the cloudfoundry API"
	errGetCreds          = "cannot get credentials"
	errRead              = "cannot read cloudfoundry SpaceMembers"
	errCreate            = "cannot create cloudfoundry SpaceMembers"
	errUpdate            = "cannot update cloudfoundry SpaceMembers"
	errDelete            = "cannot delete cloudfoundry SpaceMembers"
	errSpaceNotResolved  = "cannot resolve reference to Space."
	errExternalNameFmt   = "external-name '%s' is not a valid format, expected '<space-guid>/<role-type>'"
)

const externalNameSeparator = "/"

func composeExternalName(spaceGUID, roleType string) string {
	return spaceGUID + externalNameSeparator + roleType
}

func parseExternalName(externalName string) (spaceGUID, roleType string, err error) {
	parts := strings.SplitN(externalName, externalNameSeparator, 3)
	if len(parts) != 2 {
		return "", "", errors.New(fmt.Sprintf(errExternalNameFmt, externalName))
	}
	spaceGUID = parts[0]
	roleType = parts[1]
	if spaceGUID == "" || roleType == "" {
		return "", "", errors.New(fmt.Sprintf(errExternalNameFmt, externalName))
	}
	return spaceGUID, roleType, nil
}

func isOldExternalNameFormat(externalName string) bool {
	return clients.IsValidGUID(externalName)
}

// Setup adds a controller that reconciles managed resources SpaceMembers.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.SpaceMembersGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	options := []managed.ReconcilerOption{

		managed.WithExternalConnecter(&connector{
			kube:        mgr.GetClient(),
			usage:       resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1beta1.ProviderConfigUsage{}),
			newClientFn: members.NewClient}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...),
		managed.WithPollInterval(o.PollInterval),
		managed.WithInitializers(),
	}

	if o.Features.Enabled(features.EnableBetaManagementPolicies) {
		options = append(options, managed.WithManagementPolicies())
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.SpaceMembersGroupVersionKind),
		options...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.SpaceMembers{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube        k8s.Client
	usage       resource.Tracker
	newClientFn func(*config.Config) (*members.Client, error)
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	if _, ok := mg.(*v1alpha1.SpaceMembers); !ok {
		return nil, errors.New(errWrongKind)
	}

	config, err := clients.GetCredentialConfig(ctx, c.kube, mg)
	if err != nil {
		return nil, errors.Wrap(err, errGetProviderConfig)
	}

	client, err := c.newClientFn(config)
	if err != nil {
		return nil, errors.Wrap(err, errGetClient)
	}

	return &external{client: client}, nil
}

// Disconnect implements the managed.ExternalClient interface
func (c *external) Disconnect(ctx context.Context) error {
	// No cleanup needed for Cloud Foundry client
	return nil
}

type spaceMemberClient interface {
	ObserveSpaceMembers(ctx context.Context, cr *v1alpha1.SpaceMembers) (*v1alpha1.RoleAssignments, error)
	AssignSpaceMembers(ctx context.Context, cr *v1alpha1.SpaceMembers) (*v1alpha1.RoleAssignments, error)
	UpdateSpaceMembers(ctx context.Context, cr *v1alpha1.SpaceMembers) (*v1alpha1.RoleAssignments, error)
	DeleteSpaceMembers(ctx context.Context, cr *v1alpha1.SpaceMembers) error
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	client spaceMemberClient
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.SpaceMembers)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errWrongKind)
	}

	// Check that reference to Space is resolved
	if cr.Spec.ForProvider.Space == nil {
		return managed.ExternalObservation{}, errors.New(errSpaceNotResolved)
	}

	externalName := meta.GetExternalName(cr)

	// Step 1: Handle empty or default external-name
	if externalName == "" || externalName == cr.GetName() {
		meta.SetExternalName(cr, composeExternalName(*cr.Spec.ForProvider.Space, cr.Spec.ForProvider.RoleType))
		return managed.ExternalObservation{
			ResourceExists:          false,
			ResourceLateInitialized: true,
		}, nil
	}

	// Step 2: Migrate legacy format (bare space GUID)
	if isOldExternalNameFormat(externalName) {
		meta.SetExternalName(cr, composeExternalName(externalName, cr.Spec.ForProvider.RoleType))
		return managed.ExternalObservation{
			ResourceExists:          false,
			ResourceLateInitialized: true,
		}, nil
	}

	// Step 3: Validate compound key format
	spaceGUID, _, err := parseExternalName(externalName)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	// Step 4: Validate GUID portion of compound key
	if !clients.IsValidGUID(spaceGUID) {
		return managed.ExternalObservation{}, errors.New(fmt.Sprintf("space GUID '%s' in external-name is not a valid UUID format", spaceGUID))
	}

	observed, err := c.client.ObserveSpaceMembers(ctx, cr)

	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errRead)
	}

	if observed == nil {
		return managed.ExternalObservation{
			ResourceExists:   cr.Status.AtProvider.AssignedRoles != nil,
			ResourceUpToDate: false,
		}, nil
	}

	cr.Status.AtProvider.AssignedRoles = observed.AssignedRoles
	cr.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.SpaceMembers)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errWrongKind)
	}

	cr.SetConditions(xpv1.Creating())

	created, err := c.client.AssignSpaceMembers(ctx, cr)

	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	meta.SetExternalName(cr, composeExternalName(*cr.Spec.ForProvider.Space, cr.Spec.ForProvider.RoleType))

	// Collection resource — no single CF GUID, so set status directly.
	cr.Status.AtProvider.AssignedRoles = created.AssignedRoles

	return managed.ExternalCreation{
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.SpaceMembers)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errWrongKind)
	}

	updated, err := c.client.UpdateSpaceMembers(ctx, cr)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
	}

	meta.SetExternalName(cr, composeExternalName(*cr.Spec.ForProvider.Space, cr.Spec.ForProvider.RoleType))

	cr.Status.AtProvider.AssignedRoles = updated.AssignedRoles

	return managed.ExternalUpdate{
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.SpaceMembers)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errWrongKind)
	}

	cr.SetConditions(xpv1.Deleting())

	// nothing to delete
	if len(cr.Status.AtProvider.AssignedRoles) == 0 {
		return managed.ExternalDelete{}, nil
	}

	err := c.client.DeleteSpaceMembers(ctx, cr)
	if err != nil {
		// ADR: 404 not found means already deleted — not an error
		if clients.ErrorIsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDelete)
	}

	return managed.ExternalDelete{}, nil
}
