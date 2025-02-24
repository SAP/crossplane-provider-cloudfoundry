package role

import (
	"context"
	"time"

	cfv3 "github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha2"
)

// SpaceRoleType converts string to SpaceRoleType enum
func SpaceRoleType(roleType v1alpha2.SpaceRoleType) resource.SpaceRoleType {
	switch roleType {
	case v1alpha2.SpaceAuditor:
		return resource.SpaceRoleAuditor
	case v1alpha2.SpaceDeveloper:
		return resource.SpaceRoleDeveloper
	case v1alpha2.SpaceManager:
		return resource.SpaceRoleManager
	case v1alpha2.SpaceSupporter:
		return resource.SpaceRoleSupporter
	default:
		return resource.SpaceRoleNone
	}
}

// GetSpaceRole returns the role of a user in a space if the role matches the spec
func GetSpaceRole(ctx context.Context, client Role, spec *v1alpha2.SpaceRoleParameters) (*resource.Role, error) {
	// list all users with the role
	roles, users, err := client.ListIncludeUsersAll(ctx, NewSpaceRoleListOptions(spec))
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

// NewSpaceRoleListOptions returns a list options for the given SpaceRoleParameters
func NewSpaceRoleListOptions(spec *v1alpha2.SpaceRoleParameters) *cfv3.RoleListOptions {
	opts := cfv3.NewRoleListOptions()

	if spec.Space != nil {
		opts.SpaceGUIDs.EqualTo(*spec.Space)
	}

	var emptySpaceRoleType v1alpha2.SpaceRoleType
	if spec.Type != emptySpaceRoleType {
		opts.WithSpaceRoleType(SpaceRoleType(spec.Type))
	}

	if spec.User != nil {
		opts.UserGUIDs.EqualTo(*spec.User)
	}
	return opts
}

// GenerateSpaceRoleObservation takes an Role resource and returns *OrgRoleObservation.
func GenerateSpaceRoleObservation(o *resource.Role) v1alpha2.SpaceRoleObservation {
	obs := v1alpha2.SpaceRoleObservation{
		ID:        ptr.To(o.GUID),
		User:      &o.Relationships.User.Data.GUID,
		Type:      &o.Type,
		CreatedAt: ptr.To(o.CreatedAt.Format(time.RFC3339)),
		UpdatedAt: ptr.To(o.UpdatedAt.Format(time.RFC3339)),
	}
	return obs
}

// LateInitializeSpaceRole fills the unassigned fields with values from a Role resource.
func LateInitializeSpaceRole(spec *v1alpha2.SpaceRoleParameters, from *resource.Role) {
	if spec.User == nil {
		spec.User = &from.Relationships.User.Data.GUID
	}

	// TODO: ADD labels and annotations
}

// IsSpaceRoleUpToDate checks whether current state is up-to-date compared to the given
// set of parameters.
func IsSpaceRoleUpToDate(spec v1alpha2.OrgRoleParameters, observed *resource.Role) bool {
	return true
}
