package members

import (
	"context"
	"strings"

	cfv3 "github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"

	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/clients"
)

func toMemberKey(u *resource.User) string {
	user := v1alpha1.Member{Username: u.Username, Origin: u.Origin}
	return user.Key()
}

func orgRoleType(roleType v1alpha1.OrgRoleType) resource.OrganizationRoleType {
	switch roleType {
	case v1alpha1.OrgAuditor:
		return resource.OrganizationRoleAuditor
	case v1alpha1.OrgManager:
		return resource.OrganizationRoleManager
	case v1alpha1.OrgBillingManager:
		return resource.OrganizationRoleBillingManager
	case v1alpha1.OrgUser:
		return resource.OrganizationRoleUser
	default:
		return resource.OrganizationRoleNone
	}
}

func spaceRoleType(roleType v1alpha1.SpaceRoleType) resource.SpaceRoleType {
	switch roleType {
	case v1alpha1.SpaceAuditor:
		return resource.SpaceRoleAuditor
	case v1alpha1.SpaceManager:
		return resource.SpaceRoleManager
	case v1alpha1.SpaceDeveloper:
		return resource.SpaceRoleDeveloper
	case v1alpha1.SpaceSupporter:
		return resource.SpaceRoleSupporter
	default:
		return resource.SpaceRoleNone
	}
}
func newSpaceRoleListOptions(cr *v1alpha1.SpaceMembers) *cfv3.RoleListOptions {
	opts := cfv3.NewRoleListOptions()
	opts.SpaceGUIDs.EqualTo(*cr.Spec.ForProvider.Space)
	opts.WithSpaceRoleType(spaceRoleType(cr.Spec.ForProvider.RoleType))
	return opts
}

func newOrgRoleListOptions(cr *v1alpha1.OrgMembers) *cfv3.RoleListOptions {
	opts := cfv3.NewRoleListOptions()
	opts.OrganizationGUIDs.EqualTo(*cr.Spec.ForProvider.Org)
	opts.WithOrganizationRoleType(orgRoleType(cr.Spec.ForProvider.RoleType))
	return opts
}

// CreateOrganizationRoleByUsername assigns a user to a role by role type
func (c *Client) CreateOrganizationRoleByUsername(ctx context.Context, org string, roleType v1alpha1.OrgRoleType, username string, origin string) (*resource.Role, error) {
	return c.Roles.CreateOrganizationRoleWithUsername(ctx, org, username, orgRoleType(roleType), origin)
}

// CreateSpaceRoleByUsername assigns a user to a space role by role type
func (c *Client) CreateSpaceRoleByUsername(ctx context.Context, space string, roleType v1alpha1.SpaceRoleType, username string, origin string) (*resource.Role, error) {
	s, err := c.Spaces.Get(ctx, space)
	if err != nil {
		return nil, err
	}

	// blind create to ensure user has a role in the org
	if _, err := c.CreateOrganizationRoleByUsername(ctx, s.Relationships.Organization.Data.GUID, v1alpha1.OrgUser, username, origin); err != nil {

		if strings.Contains(err.Error(), "No user exists") {
			// TODO: create user once API is available. For now, return error
			return nil, err
		}
		// else do nothing if the user already has a role in the org
	}

	return c.Roles.CreateSpaceRoleWithUsername(ctx, space, username, spaceRoleType(roleType), origin)
}

// ListUsersWithRole returns a list of users with a specific role
func (c *Client) ListUsersWithRole(ctx context.Context, opts *cfv3.RoleListOptions) (map[string]string, error) {
	// list of all org users with the specified role type
	roles, users, err := c.Roles.ListIncludeUsersAll(ctx, opts)
	if err != nil {
		return nil, err
	}

	roleMap := make(map[string]string)
	for _, r := range roles {
		roleMap[r.Relationships.User.Data.GUID] = r.GUID
	}
	m := make(map[string]string)
	for _, u := range users {
		m[toMemberKey(u)] = roleMap[u.GUID]
	}
	return m, nil
}

// RemoveUsersFromRole removes all roles managed by the given CR.
func (c *Client) RemoveUsersFromRole(ctx context.Context, roleMembers map[string]string) error {
	for _, role := range roleMembers {
		if err := c.DeleteRole(ctx, role); err != nil {
			return err
		}
	}
	return nil
}

// DeleteRole deletes a role object
func (c *Client) DeleteRole(ctx context.Context, role string) error {
	_, err := c.Roles.Delete(ctx, role)
	// suppress not_found
	if err != nil && !clients.ErrorIsNotFound(err) {
		return err
	}
	return nil
}
