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
	errRoleTypeRequired  = "roleType is required when external-name is not set: specify spec.forProvider.roleType or set a valid <space-guid>/<role-type> external-name"
	errExternalNameFmt   = "external-name '%s' is not a valid format, expected '<space-guid>/<role-type>'"
	errInvalidRoleType   = "role type '%s' is not a valid space role type"
)

const externalNameSeparator = "/"

func composeExternalName(spaceGUID, roleType string) string {
	return spaceGUID + externalNameSeparator + canonicalizeRoleType(roleType)
}

// canonicalizeRoleType normalizes role type aliases to their singular canonical form.
// Both Manager/Managers, Developer/Developers, etc. map to the same CF role;
// the external-name must use the canonical form to avoid identity conflicts.
func canonicalizeRoleType(roleType string) string {
	switch roleType {
	case v1alpha1.SpaceAuditors:
		return v1alpha1.SpaceAuditor
	case v1alpha1.SpaceDevelopers:
		return v1alpha1.SpaceDeveloper
	case v1alpha1.SpaceManagers:
		return v1alpha1.SpaceManager
	case v1alpha1.SpaceSupporters:
		return v1alpha1.SpaceSupporter
	default:
		return roleType
	}
}

// isValidSpaceRoleType checks whether the role type (in any alias form) maps to a valid CF space role.
// Used to validate external-name role segments before they are used in identity resolution.
func isValidSpaceRoleType(roleType string) bool {
	switch canonicalizeRoleType(roleType) {
	case v1alpha1.SpaceAuditor, v1alpha1.SpaceDeveloper, v1alpha1.SpaceManager, v1alpha1.SpaceSupporter:
		return true
	default:
		return false
	}
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
	if !isValidSpaceRoleType(roleType) {
		return "", "", errors.New(fmt.Sprintf(errInvalidRoleType, roleType))
	}
	return spaceGUID, canonicalizeRoleType(roleType), nil
}

func isOldExternalNameFormat(externalName string) bool {
	return clients.IsValidGUID(externalName)
}

func hasCompoundExternalName(externalName, resourceName string) bool {
	return externalName != "" && externalName != resourceName && !isOldExternalNameFormat(externalName)
}

func validateIdentityConflict(cr *v1alpha1.SpaceMembers, spaceGUID, roleType string) error {
	if !hasCompoundExternalName(meta.GetExternalName(cr), cr.GetName()) {
		return nil
	}
	if cr.Spec.ForProvider.Space != nil && *cr.Spec.ForProvider.Space != spaceGUID {
		return errors.Errorf("identity conflict: external-name space (%s) differs from spec (%s)", spaceGUID, *cr.Spec.ForProvider.Space)
	}
	if cr.Spec.ForProvider.RoleType != "" && canonicalizeRoleType(cr.Spec.ForProvider.RoleType) != roleType {
		return errors.Errorf("identity conflict: external-name role type (%s) differs from spec (%s)", roleType, cr.Spec.ForProvider.RoleType)
	}
	return nil
}

func buildObservation(lateInitialized, exists bool, observed *v1alpha1.RoleAssignments) managed.ExternalObservation {
	if !exists {
		return managed.ExternalObservation{
			ResourceExists:          false,
			ResourceLateInitialized: lateInitialized,
		}
	}
	if observed == nil {
		return managed.ExternalObservation{
			ResourceExists:          true,
			ResourceUpToDate:        false,
			ResourceLateInitialized: lateInitialized,
		}
	}
	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        true,
		ResourceLateInitialized: lateInitialized,
	}
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
	ObserveSpaceMembers(ctx context.Context, spaceGUID, roleType string, cr *v1alpha1.SpaceMembers) (*v1alpha1.RoleAssignments, bool, error)
	AssignSpaceMembers(ctx context.Context, spaceGUID, roleType string, cr *v1alpha1.SpaceMembers) (*v1alpha1.RoleAssignments, error)
	UpdateSpaceMembers(ctx context.Context, spaceGUID, roleType string, cr *v1alpha1.SpaceMembers) (*v1alpha1.RoleAssignments, error)
	DeleteSpaceMembers(ctx context.Context, spaceGUID, roleType string, cr *v1alpha1.SpaceMembers) error
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	client spaceMemberClient
}

func resolveIdentity(cr *v1alpha1.SpaceMembers) (spaceGUID, roleType string, lateInitialized bool, err error) {
	externalName := meta.GetExternalName(cr)

	switch {
	case externalName == "" || externalName == cr.GetName():
		if cr.Spec.ForProvider.Space == nil {
			return "", "", false, errors.New(errSpaceNotResolved)
		}
		if !isValidSpaceRoleType(cr.Spec.ForProvider.RoleType) {
			return "", "", false, errors.New(errRoleTypeRequired)
		}
		return *cr.Spec.ForProvider.Space, canonicalizeRoleType(cr.Spec.ForProvider.RoleType), true, nil
	case isOldExternalNameFormat(externalName):
		if !isValidSpaceRoleType(cr.Spec.ForProvider.RoleType) {
			return "", "", false, errors.New(errRoleTypeRequired)
		}
		return externalName, canonicalizeRoleType(cr.Spec.ForProvider.RoleType), true, nil
	default:
		spaceGUID, roleType, err := parseExternalName(externalName)
		if err != nil {
			return "", "", false, err
		}
		if !clients.IsValidGUID(spaceGUID) {
			return "", "", false, errors.New(fmt.Sprintf("space GUID '%s' in external-name is not a valid UUID format", spaceGUID))
		}
		return spaceGUID, roleType, false, nil
	}
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.SpaceMembers)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errWrongKind)
	}

	spaceGUID, roleType, lateInitialized, err := resolveIdentity(cr)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	if err := validateIdentityConflict(cr, spaceGUID, roleType); err != nil {
		return managed.ExternalObservation{}, err
	}

	if lateInitialized {
		meta.SetExternalName(cr, composeExternalName(spaceGUID, roleType))
	}

	observed, exists, err := c.client.ObserveSpaceMembers(ctx, spaceGUID, roleType, cr)

	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errRead)
	}

	if observed != nil {
		cr.Status.AtProvider.AssignedRoles = observed.AssignedRoles
		cr.SetConditions(xpv1.Available())
	}

	observation := buildObservation(lateInitialized, exists, observed)

	// Under Lax enforcement, once our tracked roles are removed during deletion, the external resource is gone
	if meta.WasDeleted(cr) && cr.Spec.ForProvider.EnforcementPolicy != "Strict" && len(cr.Status.AtProvider.AssignedRoles) == 0 {
		observation.ResourceExists = false
	}

	return observation, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.SpaceMembers)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errWrongKind)
	}

	cr.SetConditions(xpv1.Creating())

	spaceGUID, roleType, _, err := resolveIdentity(cr)
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	created, err := c.client.AssignSpaceMembers(ctx, spaceGUID, roleType, cr)

	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	meta.SetExternalName(cr, composeExternalName(spaceGUID, roleType))

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

	spaceGUID, roleType, _, err := resolveIdentity(cr)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	updated, err := c.client.UpdateSpaceMembers(ctx, spaceGUID, roleType, cr)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
	}

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

	// Lax resources only remove the roles tracked in status. If nothing is tracked,
	// deletion is already complete and we should not block on resolving identity.
	if cr.Spec.ForProvider.EnforcementPolicy != "Strict" && len(cr.Status.AtProvider.AssignedRoles) == 0 {
		return managed.ExternalDelete{}, nil
	}

	spaceGUID, roleType, _, err := resolveIdentity(cr)
	if err != nil {
		return managed.ExternalDelete{}, err
	}

	err = c.client.DeleteSpaceMembers(ctx, spaceGUID, roleType, cr)
	if err != nil {
		// ADR: 404 not found means already deleted — not an error
		if clients.ErrorIsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDelete)
	}

	return managed.ExternalDelete{}, nil
}
