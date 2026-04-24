package orgmembers

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
	errWrongKind         = "Managed resource is not an OrgMembers kind"
	errTrackUsage        = "cannot track usage"
	errGetProviderConfig = "cannot get ProviderConfig or resolve credential references"
	errGetClient         = "cannot create a client to talk to the cloudfoundry API"
	errGetCreds          = "cannot get credentials"
	errRead              = "cannot read cloudfoundry OrgMembers"
	errCreate            = "cannot create cloudfoundry OrgMembers"
	errUpdate            = "cannot update cloudfoundry OrgMembers"
	errDelete            = "cannot delete cloudfoundry OrgMembers"
	errOrgNotResolved    = "org reference is not resolved."
	errRoleTypeRequired  = "roleType is required when external-name is not set: specify spec.forProvider.roleType or set a valid <org-guid>/<role-type> external-name"
	errExternalNameFmt   = "external-name '%s' is not a valid format, expected '<org-guid>/<role-type>'"
	errInvalidRoleType   = "role type '%s' is not a valid org role type"
)

const externalNameSeparator = "/"

func composeExternalName(orgGUID, roleType string) string {
	return orgGUID + externalNameSeparator + canonicalizeRoleType(roleType)
}

func parseExternalName(externalName string) (orgGUID, roleType string, err error) {
	parts := strings.SplitN(externalName, externalNameSeparator, 3)
	if len(parts) != 2 {
		return "", "", errors.New(fmt.Sprintf(errExternalNameFmt, externalName))
	}
	orgGUID = parts[0]
	roleType = parts[1]
	if orgGUID == "" || roleType == "" {
		return "", "", errors.New(fmt.Sprintf(errExternalNameFmt, externalName))
	}
	if !isValidOrgRoleType(roleType) {
		return "", "", errors.New(fmt.Sprintf(errInvalidRoleType, roleType))
	}
	return orgGUID, canonicalizeRoleType(roleType), nil
}

func isOldExternalNameFormat(externalName string) bool {
	return strings.Contains(externalName, "@")
}

// canonicalizeRoleType normalizes role type aliases to their singular canonical form.
// Both Manager/Managers, User/Users, etc. map to the same CF role;
// the external-name must use the canonical form to avoid identity conflicts.
func canonicalizeRoleType(roleType string) string {
	switch roleType {
	case v1alpha1.OrgAuditors:
		return v1alpha1.OrgAuditor
	case v1alpha1.OrgManagers:
		return v1alpha1.OrgManager
	case v1alpha1.OrgBillingManagers:
		return v1alpha1.OrgBillingManager
	case v1alpha1.OrgUsers:
		return v1alpha1.OrgUser
	default:
		return roleType
	}
}

// isValidOrgRoleType checks whether the role type (in any alias form) maps to a valid CF org role.
// Used to validate external-name role segments before they are used in identity resolution.
func isValidOrgRoleType(roleType string) bool {
	switch canonicalizeRoleType(roleType) {
	case v1alpha1.OrgAuditor, v1alpha1.OrgManager, v1alpha1.OrgBillingManager, v1alpha1.OrgUser:
		return true
	default:
		return false
	}
}

// Setup adds a controller that reconciles managed resources OrgMembers.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.OrgMembersGroupKind)

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
		resource.ManagedKind(v1alpha1.OrgMembersGroupVersionKind),
		options...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.OrgMembers{}).
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
	if _, ok := mg.(*v1alpha1.OrgMembers); !ok {
		return nil, errors.New(errWrongKind)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackUsage)
	}

	cfg, err := clients.GetCredentialConfig(ctx, c.kube, mg)
	if err != nil {
		return nil, errors.Wrap(err, errGetProviderConfig)
	}

	client, err := c.newClientFn(cfg)
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

type orgMemberClient interface {
	ObserveOrgMembers(ctx context.Context, orgGUID, roleType string, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, bool, error)
	AssignOrgMembers(ctx context.Context, orgGUID, roleType string, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, error)
	UpdateOrgMembers(ctx context.Context, orgGUID, roleType string, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, error)
	DeleteOrgMembers(ctx context.Context, orgGUID, roleType string, cr *v1alpha1.OrgMembers) error
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	client orgMemberClient
}

func resolveIdentity(cr *v1alpha1.OrgMembers) (orgGUID, roleType string, lateInitialized bool, err error) {
	externalName := meta.GetExternalName(cr)

	switch {
	case externalName == "" || externalName == cr.GetName():
		return resolveIdentityFromSpec(cr)
	case isOldExternalNameFormat(externalName):
		return resolveLegacyIdentity(externalName)
	default:
		return resolveCompoundIdentity(externalName)
	}
}

func resolveIdentityFromSpec(cr *v1alpha1.OrgMembers) (orgGUID, roleType string, lateInitialized bool, err error) {
	if cr.Spec.ForProvider.Org == nil {
		return "", "", false, errors.New(errOrgNotResolved)
	}
	if !isValidOrgRoleType(cr.Spec.ForProvider.RoleType) {
		return "", "", false, errors.New(errRoleTypeRequired)
	}
	return *cr.Spec.ForProvider.Org, canonicalizeRoleType(cr.Spec.ForProvider.RoleType), true, nil
}

func resolveLegacyIdentity(externalName string) (orgGUID, roleType string, lateInitialized bool, err error) {
	parts := strings.SplitN(externalName, "@", 2)
	if len(parts) != 2 {
		return "", "", false, errors.New(fmt.Sprintf("legacy external-name '%s' has invalid format, expected 'RoleType@OrgGUID'", externalName))
	}

	legacyOrgGUID, legacyRoleType := parts[1], parts[0]
	if !clients.IsValidGUID(legacyOrgGUID) {
		return "", "", false, errors.New(fmt.Sprintf("legacy external-name '%s' contains invalid org GUID '%s'", externalName, legacyOrgGUID))
	}
	if !isValidOrgRoleType(legacyRoleType) {
		return "", "", false, errors.New(fmt.Sprintf(errInvalidRoleType, legacyRoleType))
	}

	return legacyOrgGUID, canonicalizeRoleType(legacyRoleType), true, nil
}

func resolveCompoundIdentity(externalName string) (orgGUID, roleType string, lateInitialized bool, err error) {
	orgGUID, roleType, err = parseExternalName(externalName)
	if err != nil {
		return "", "", false, err
	}
	if !clients.IsValidGUID(orgGUID) {
		return "", "", false, errors.New(fmt.Sprintf("org GUID '%s' in external-name is not a valid UUID format", orgGUID))
	}
	return orgGUID, roleType, false, nil
}

func hasCompoundExternalName(externalName, resourceName string) bool {
	return externalName != "" && externalName != resourceName && !isOldExternalNameFormat(externalName)
}

func validateIdentityConflict(cr *v1alpha1.OrgMembers, orgGUID, roleType string) error {
	if !hasCompoundExternalName(meta.GetExternalName(cr), cr.GetName()) {
		return nil
	}
	if cr.Spec.ForProvider.Org != nil && *cr.Spec.ForProvider.Org != orgGUID {
		return errors.Errorf("identity conflict: external-name org (%s) differs from spec (%s)", orgGUID, *cr.Spec.ForProvider.Org)
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

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.OrgMembers)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errWrongKind)
	}

	orgGUID, roleType, lateInitialized, err := resolveIdentity(cr)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	if err := validateIdentityConflict(cr, orgGUID, roleType); err != nil {
		return managed.ExternalObservation{}, err
	}

	if lateInitialized {
		meta.SetExternalName(cr, composeExternalName(orgGUID, roleType))
	}

	observed, exists, err := c.client.ObserveOrgMembers(ctx, orgGUID, roleType, cr)

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
	cr, ok := mg.(*v1alpha1.OrgMembers)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errWrongKind)
	}

	// TODO: checking conflicting CR that `strictly` enforces the same role on the same
	cr.SetConditions(xpv1.Creating())

	orgGUID, roleType, _, err := resolveIdentity(cr)
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	created, err := c.client.AssignOrgMembers(ctx, orgGUID, roleType, cr)

	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	meta.SetExternalName(cr, composeExternalName(orgGUID, roleType))

	// Collection resource — no single CF GUID, so set status directly.
	cr.Status.AtProvider.AssignedRoles = created.AssignedRoles

	return managed.ExternalCreation{
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.OrgMembers)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errWrongKind)
	}

	orgGUID, roleType, _, err := resolveIdentity(cr)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	updated, err := c.client.UpdateOrgMembers(ctx, orgGUID, roleType, cr)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdate)
	}

	// External-name is immutable after initial creation/migration — do NOT rewrite it.
	cr.Status.AtProvider.AssignedRoles = updated.AssignedRoles

	return managed.ExternalUpdate{
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.OrgMembers)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errWrongKind)
	}

	cr.SetConditions(xpv1.Deleting())

	// Lax resources only remove the roles tracked in status. If nothing is tracked,
	// deletion is already complete and we should not block on resolving identity.
	if cr.Spec.ForProvider.EnforcementPolicy != "Strict" && len(cr.Status.AtProvider.AssignedRoles) == 0 {
		return managed.ExternalDelete{}, nil
	}

	// TODO: make sure there is at least one manager of the org?
	// TODO: In case of deletion error for some roles, this resource will stuck in a false status (READY=false and SYNCED=false). We need a strategy to handle this.
	// 		 e.g., organization_user role cannot be deleted if the user has role in some spaces in the same org.

	orgGUID, roleType, _, err := resolveIdentity(cr)
	if err != nil {
		return managed.ExternalDelete{}, err
	}

	err = c.client.DeleteOrgMembers(ctx, orgGUID, roleType, cr)
	if err != nil {
		// ADR: 404 not found means already deleted — not an error
		if clients.ErrorIsNotFound(err) {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, errors.Wrap(err, errDelete)
	}

	return managed.ExternalDelete{}, nil
}
