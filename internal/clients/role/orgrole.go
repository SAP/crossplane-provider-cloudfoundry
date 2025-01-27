package role

import (
	"context"
	"time"

	cfv3 "github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"k8s.io/utils/ptr"

	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/resources/v1alpha2"
)

// OrgRoleType converts string to OrganizationRoleType enum
func OrgRoleType(roleType v1alpha2.OrgRoleType) resource.OrganizationRoleType {
	switch roleType {
	case v1alpha2.OrgAuditor:
		return resource.OrganizationRoleAuditor
	case v1alpha2.OrgManager:
		return resource.OrganizationRoleManager
	case v1alpha2.OrgBillingManager:
		return resource.OrganizationRoleBillingManager
	case v1alpha2.OrgUser:
		return resource.OrganizationRoleUser
	default:
		return resource.OrganizationRoleNone
	}
}

// GetOrgRole returns the role of a user in an organization if the role matches the spec
func GetOrgRole(ctx context.Context, client Role, spec *v1alpha2.OrgRoleParameters) (*resource.Role, error) {
	// list all users with the role
	roles, users, err := client.ListIncludeUsersAll(ctx, NewOrgRoleListOptions(spec))
	if err != nil {
		return nil, err
	}

	var noUserRelation resource.ToOneRelationship
	// list of all org users with the specified role type
	roleMap := make(map[string]*resource.Role)
	for _, ro := range roles {
		if ro.Relationships.User == noUserRelation {
			continue
		}
		roleMap[ro.Relationships.User.Data.GUID] = ro
	}

	m := make(map[string]*resource.Role)
	for _, u := range users {
		m[toMemberKey(u)] = roleMap[u.GUID]
	}

	// check if the user is included in the list of users with the role
	member := toMember(ptr.Deref(spec.Username, ""), ptr.Deref(spec.Origin, "sap.ids"))
	r, ok := m[member.key()]
	if !ok {
		return nil, nil
	}
	return r, nil
}

// NewOrgRoleListOptions returns a list options for the given OrgRoleParameters
func NewOrgRoleListOptions(spec *v1alpha2.OrgRoleParameters) *cfv3.RoleListOptions {
	opts := cfv3.NewRoleListOptions()

	if spec.Org != nil {
		opts.OrganizationGUIDs.EqualTo(*spec.Org)
	}

	var emptyOrgRoleType v1alpha2.OrgRoleType
	if spec.Type != emptyOrgRoleType {
		opts.WithOrganizationRoleType(OrgRoleType(spec.Type))
	}

	if spec.User != nil {
		opts.UserGUIDs.EqualTo(*spec.User)
	}
	return opts
}

// GenerateOrgRoleObservation takes an Role resource and returns *OrgRoleObservation.
func GenerateOrgRoleObservation(o *resource.Role) v1alpha2.OrgRoleObservation {
	obs := v1alpha2.OrgRoleObservation{
		ID:        ptr.To(o.GUID),
		User:      &o.Relationships.User.Data.GUID,
		Type:      &o.Type,
		CreatedAt: ptr.To(o.CreatedAt.Format(time.RFC3339)),
		UpdatedAt: ptr.To(o.UpdatedAt.Format(time.RFC3339)),
	}
	return obs
}

// LateInitializeOrgRole fills the unassigned fields with values from a Role resource.
func LateInitializeOrgRole(spec v1alpha2.OrgRoleParameters, from *resource.Role) {
	if spec.User == nil {
		spec.User = &from.Relationships.User.Data.GUID
	}

	// TODO: ADD labels and annotations
}

// IsOrgRoleUpToDate checks whether current state is up-to-date compared to the given
// set of parameters.
func IsOrgRoleUpToDate(spec v1alpha2.OrgRoleParameters, observed *resource.Role) bool {
	return true
}
