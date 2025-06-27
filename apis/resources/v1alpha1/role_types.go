package v1alpha1

import "strings"

const (
	// OrgAuditor is the role type for organization auditor
	OrgAuditor = "Auditor"
	// OrgManager is the role type for organization manager
	OrgManager = "Manager"
	// OrgBillingManager is the role type for organization billing manager
	OrgBillingManager = "BillingManager"
	// OrgUser is the role type for organization user
	OrgUser = "User"

	// SpaceAuditor is the role type for space auditor
	SpaceAuditor = "Auditor"
	// SpaceManager is the role type for space manager
	SpaceManager = "Manager"
	// SpaceSupporter is the role type for space supporter
	SpaceSupporter = "Supporter"
	// SpaceDeveloper is the role type for space developer
	SpaceDeveloper = "Developer"

	// backward compatibility
	OrgAuditors        = "Auditors"
	OrgManagers        = "Managers"
	OrgBillingManagers = "BillingManagers"
	OrgUsers           = "Users"
	SpaceAuditors      = "Auditors"
	SpaceManagers      = "Managers"
	SpaceSupporters    = "Supporters"
	SpaceDevelopers    = "Developers"
)

// Member identifies a user by name and origin.
type Member struct {
	// (String) Username at the identity provider.
	Username string `json:"username"`
	// (String) Origin selects the identity provider. Defaults to "sap.ids".
	// +kubebuilder:default=sap.ids
	Origin string `json:"origin,omitempty"`
}

// Key return a formatted string identifying the Member
func (u *Member) Key() string {
	// todo: default origin to "sap.ids", replace this with scim lookup
	if u.Origin == "" {
		u.Origin = "sap.ids"
	}
	// username and origin should be case insensitive / lower case
	return strings.ToLower(u.Username + " (" + u.Origin + ")")
}

// Equal compares member to other objects
func (u *Member) Equal(other interface{}) bool {
	uu, ok := other.(*Member)
	if !ok {
		return false
	}

	if u.Origin == "" {
		return u.Username == uu.Username
	}

	return u.Username == uu.Username && u.Origin == uu.Origin
}

// RoleAssignments maps members to roles.
type RoleAssignments struct {
	// (Map of String) `assignedRoles` maps a member to the GUID of the assigned Role object.
	AssignedRoles map[string]string `json:"assignedRoles,omitempty"`
}

// MemberList includes a list of members and an enforcement policy for role assignment.
type MemberList struct {
	// (List of Attributes) List of members (usernames) to assign as org members with the specified role type. Defaults to empty list.
	Members []*Member `json:"members"`

	// (String) Set to `Lax` to enforce that the role is assigned to AT LEAST those members as defined in this CR. Set to `Strict` to enforce that the role is assigned to EXACTLY those members as defined in this CR and any other members will be removed. Defaults to `Lax`.
	// +optional
	// +kubebuilder:default=Lax
	// +kubebuilder:validation:Enum=Lax;Strict
	EnforcementPolicy string `json:"enforcementPolicy,omitempty"`
}
