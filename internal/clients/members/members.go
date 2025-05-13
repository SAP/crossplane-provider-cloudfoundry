package members

import (
	"context"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
)

const (
	enforcementPolicyStrict = "Strict"
)

// AssignOrgMembers assigns org role to a set of users in an all-or-none fashion, and return a map of assigned roles.
func (c *Client) AssignOrgMembers(ctx context.Context, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, error) {
	// get all users with the role
	observed, err := c.ListUsersWithRole(ctx, newOrgRoleListOptions(cr))
	if err != nil {
		return nil, err
	}

	members := make(map[string]string)
	// make sure all defined users has the role
	for _, u := range cr.Spec.ForProvider.Members {
		user := u.Key()
		role, ok := observed[user]
		if !ok {
			r, err := c.CreateOrganizationRoleByUsername(ctx, *cr.Spec.ForProvider.Org, cr.Spec.ForProvider.RoleType, u.Username, u.Origin)
			if err != nil {
				return nil, err
			}
			role = r.GUID
		}
		members[user] = role
	}

	// in case of "Strict" delete any roles remained in the observed list.
	if cr.Spec.ForProvider.EnforcementPolicy == enforcementPolicyStrict {
		for user, role := range observed {
			if _, ok := members[user]; !ok {
				err := c.DeleteRole(ctx, role)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	// return current as observation
	return &v1alpha1.RoleAssignments{
		AssignedRoles: members,
	}, nil
}

// UpdateOrgMembers observes external state and update it according the CR specification
func (c *Client) UpdateOrgMembers(ctx context.Context, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, error) {
	// get all users with the role
	members, err := c.AssignOrgMembers(ctx, (cr))
	if err != nil {
		return nil, err
	}

	// remove any orphans in the (previously) assigned roles. Changing CR's Org, RoleType, or Members
	// can results in all or some assigned roles become unmanaged and need to be deleted.
	if cr.Status.AtProvider.AssignedRoles != nil {
		for user, role := range cr.Status.AtProvider.AssignedRoles {
			if role != members.AssignedRoles[user] {
				if err := c.DeleteRole(ctx, role); err != nil {
					return nil, err
				}
			}
		}
	}

	// return current as observation
	return members, nil
}

// ObserveOrgMembers generates external state for the managed resources based on CR specification.
// If the observed state is not consistent with CR, return a nil observation together with an error.
func (c *Client) ObserveOrgMembers(ctx context.Context, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, error) {
	// sync every currently assigned role and remove it from members list if it no longer exists
	for user, role := range cr.Status.AtProvider.AssignedRoles {
		_, err := c.Roles.Get(ctx, role)
		if err != nil && clients.ErrorIsNotFound(err) {
			delete(cr.Status.AtProvider.AssignedRoles, user)
		}
	}
	// get all users with the role
	observed, err := c.ListUsersWithRole(ctx, newOrgRoleListOptions(cr))
	if err != nil {
		return nil, err
	}

	return generateOrgMemberObservation(observed, cr), nil
}

// isOrgMemberUpToDate checks if observation is consistent with CR
func generateOrgMemberObservation(observed map[string]string, cr *v1alpha1.OrgMembers) *v1alpha1.RoleAssignments {
	members := make(map[string]string)
	// check if all defined users has the role
	for _, u := range cr.Spec.ForProvider.Members {
		user := u.Key()
		r, ok := observed[user]
		if !ok {
			return nil
		}
		members[user] = r
	}

	// check orphans in the (previously) assigned roles. This can happen if a user is removed from
	// the defined list, or org and/or role_type changes.
	for user, role := range cr.Status.AtProvider.AssignedRoles {
		if role != members[user] {
			return nil
		}
	}

	// in case of "Strict", check orphans in the observed roles, due to external modification.
	if cr.Spec.ForProvider.EnforcementPolicy == enforcementPolicyStrict {
		if len(observed) != len(members) {
			return nil
		}
	}

	return &v1alpha1.RoleAssignments{AssignedRoles: members}
}

// DeleteOrgMembers remove external org role resources managed by this CR
func (c *Client) DeleteOrgMembers(ctx context.Context, cr *v1alpha1.OrgMembers) error {
	fp := cr.Spec.ForProvider
	// if strict, remove all users from the role
	if fp.EnforcementPolicy == enforcementPolicyStrict {
		allUsersWithRole, err := c.ListUsersWithRole(ctx, newOrgRoleListOptions(cr))
		if err != nil {
			return err
		}
		return c.RemoveUsersFromRole(ctx, allUsersWithRole)
	}
	// otherwise, remove just the members
	return c.RemoveUsersFromRole(ctx, cr.Status.AtProvider.AssignedRoles)
}

// AssignSpaceMembers assigns Space Role for the given list of users
func (c *Client) AssignSpaceMembers(ctx context.Context, cr *v1alpha1.SpaceMembers) (*v1alpha1.RoleAssignments, error) {
	// get all users with the role
	observed, err := c.ListUsersWithRole(ctx, newSpaceRoleListOptions(cr))
	if err != nil {
		return nil, err
	}

	members := make(map[string]string)
	// make sure all defined users has the role
	for _, u := range cr.Spec.ForProvider.Members {
		user := u.Key()
		role, ok := observed[user]
		if !ok {
			r, err := c.CreateSpaceRoleByUsername(ctx, *cr.Spec.ForProvider.Space, cr.Spec.ForProvider.RoleType, u.Username, u.Origin)
			if err != nil {
				return nil, err
			}
			role = r.GUID
		}
		members[user] = role
	}

	// in case of "Strict", remove any remaining user in the observed list from the role
	if cr.Spec.ForProvider.EnforcementPolicy == enforcementPolicyStrict {
		for user, role := range observed {
			if _, ok := members[user]; !ok {
				err := c.DeleteRole(ctx, role)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	// return current as observation
	return &v1alpha1.RoleAssignments{
		AssignedRoles: members,
	}, nil
}

// UpdateSpaceMembers observes external state and update it according the CR specification
func (c *Client) UpdateSpaceMembers(ctx context.Context, cr *v1alpha1.SpaceMembers) (*v1alpha1.RoleAssignments, error) {
	members, err := c.AssignSpaceMembers(ctx, cr)
	if err != nil {
		return nil, err
	}

	// remove any orphans in the (previously) assigned roles. This can happen if a user is removed from
	// the defined list, or org and/or role_type changes.
	if cr.Status.AtProvider.AssignedRoles != nil {
		for user, role := range cr.Status.AtProvider.AssignedRoles {
			if role != members.AssignedRoles[user] {
				if err := c.DeleteRole(ctx, role); err != nil {
					return nil, err
				}
			}
		}
	}
	// return members as observation
	return members, nil
}

// ObserveSpaceMembers generates external state for the managed resources based on CR specification.
// If the observed state is not consistent with CR, return a nil observation together with an error.
func (c *Client) ObserveSpaceMembers(ctx context.Context, cr *v1alpha1.SpaceMembers) (*v1alpha1.RoleAssignments, error) {
	// sync every currently assigned role and remove it from members list if it no longer exists
	for user, role := range cr.Status.AtProvider.AssignedRoles {
		_, err := c.Roles.Get(ctx, role)
		if err != nil && clients.ErrorIsNotFound(err) {
			delete(cr.Status.AtProvider.AssignedRoles, user)
		}
	}

	// get all users with the role
	observed, err := c.ListUsersWithRole(ctx, newSpaceRoleListOptions(cr))
	if err != nil {
		return nil, err
	}

	return generateSpaceMemberObservation(observed, cr), nil
}

func generateSpaceMemberObservation(observed map[string]string, cr *v1alpha1.SpaceMembers) *v1alpha1.RoleAssignments {
	members := make(map[string]string)
	// check if all defined users has the role
	for _, u := range cr.Spec.ForProvider.Members {
		user := u.Key()
		r, ok := observed[user]
		if !ok {
			return nil
		}
		members[user] = r
	}

	// check orphans in the (previously) assigned roles. This can happen if a user is removed from
	// the defined list, or space and/or role_type changes.
	for user, role := range cr.Status.AtProvider.AssignedRoles {
		if role != members[user] {
			return nil
		}
	}

	// in case of "Strict", check orphans in the observed roles, due to external modification.
	if cr.Spec.ForProvider.EnforcementPolicy == enforcementPolicyStrict {
		if len(observed) != len(members) {
			return nil
		}
	}
	return &v1alpha1.RoleAssignments{AssignedRoles: members}
}

// DeleteSpaceMembers removes space Role managed by the given CR.
func (c *Client) DeleteSpaceMembers(ctx context.Context, cr *v1alpha1.SpaceMembers) error {
	fp := cr.Spec.ForProvider
	// if strict, remove all users from the role
	if fp.EnforcementPolicy == enforcementPolicyStrict {
		allUsersWithRole, err := c.ListUsersWithRole(ctx, newSpaceRoleListOptions(cr))
		if err != nil {
			return err
		}
		return c.RemoveUsersFromRole(ctx, allUsersWithRole)
	}
	// otherwise, remove just the members
	return c.RemoveUsersFromRole(ctx, cr.Status.AtProvider.AssignedRoles)
}
