package role

import (
	"context"
	"errors"
	"time"

	cfv3 "github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
)

const ErrOrgNotSpecified = "Org is not specified"

// OrgRoleType converts string to OrganizationRoleType enum
func OrgRoleType(roleType string) resource.OrganizationRoleType {
	switch roleType {
	case v1alpha1.OrgAuditor, v1alpha1.OrgAuditors:
		return resource.OrganizationRoleAuditor
	case v1alpha1.OrgManager, v1alpha1.OrgManagers:
		return resource.OrganizationRoleManager
	case v1alpha1.OrgBillingManager, v1alpha1.OrgBillingManagers:
		return resource.OrganizationRoleBillingManager
	case v1alpha1.OrgUser, v1alpha1.OrgUsers:
		return resource.OrganizationRoleUser
	default:
		return resource.OrganizationRoleNone
	}
}

// GetOrgRole returns the role of a user in an organization by guid or by  matching the spec
func GetOrgRole(ctx context.Context, client Role, guid string, spec v1alpha1.OrgRoleParameters) (*resource.Role, error) {

	if clients.IsValidGUID(guid) {
		return client.Get(ctx, guid)
	}

	return findOrgRole(ctx, client, spec)
}

// findOrgRole returns the role of a user in an organization if the role matches the spec
func findOrgRole(ctx context.Context, client Role, spec v1alpha1.OrgRoleParameters) (*resource.Role, error) {
	opts, err := NewOrgRoleListOptions(spec)
	if err != nil {
		return nil, err
	}
	// list all users with the role
	roles, users, err := client.ListIncludeUsersAll(ctx, opts)
	if err != nil {
		return nil, err
	}

	return findRole(roles, users, spec.Username, ptr.Deref(spec.Origin, "sap.ids"), OrgRoleType(spec.Type).String())
}

// NewOrgRoleListOptions returns a list options for the given OrgRoleParameters
func NewOrgRoleListOptions(spec v1alpha1.OrgRoleParameters) (*cfv3.RoleListOptions, error) {
	opts := cfv3.NewRoleListOptions()

	if spec.Org == nil {
		// nolint:staticcheck
		return nil, errors.New(ErrOrgNotSpecified)
	}
	opts.OrganizationGUIDs.EqualTo(*spec.Org)

	opts.WithOrganizationRoleType(OrgRoleType(spec.Type))
	return opts, nil
}

// GenerateOrgRoleObservation takes an Role resource and returns *OrgRoleObservation.
func GenerateOrgRoleObservation(o *resource.Role) v1alpha1.OrgRoleObservation {
	obs := v1alpha1.OrgRoleObservation{
		ID:        ptr.To(o.GUID),
		User:      &o.Relationships.User.Data.GUID,
		Type:      &o.Type,
		CreatedAt: ptr.To(o.CreatedAt.Format(time.RFC3339)),
		UpdatedAt: ptr.To(o.UpdatedAt.Format(time.RFC3339)),
	}
	return obs
}
