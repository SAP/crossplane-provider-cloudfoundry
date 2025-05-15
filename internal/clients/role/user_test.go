package role

import (
	"testing"

	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

// unit test for findRole
func TestFindRole(t *testing.T) {

	role1 := &resource.Role{
		Resource: resource.Resource{GUID: "2d8b0d04-d537-4e4e-8c6f-f09ca0e7f56a"},
		Type:     "space_developer",
		Relationships: resource.RoleSpaceUserOrganizationRelationships{
			User: resource.ToOneRelationship{
				Data: &resource.Relationship{GUID: "338b0d04-d537-4e4e-8c6f-f09ca0e7f56a"},
			},
		},
	}
	role2 := &resource.Role{
		Resource: resource.Resource{GUID: "2d8b0d04-d537-4e4e-8c6f-f09ca0e7f56b"},
		Type:     "space_auditor",
		Relationships: resource.RoleSpaceUserOrganizationRelationships{
			User: resource.ToOneRelationship{
				Data: &resource.Relationship{GUID: "338b0d04-d537-4e4e-8c6f-f09ca0e7f56b"},
			},
		},
	}

	roles := []*resource.Role{
		role1, role2,
	}

	users := []*resource.User{
		{
			Resource: resource.Resource{GUID: "338b0d04-d537-4e4e-8c6f-f09ca0e7f56a"},
			Username: ptr.To("user1"),
			Origin:   ptr.To("sap.ids"),
		},
		{
			Resource: resource.Resource{GUID: "338b0d04-d537-4e4e-8c6f-f09ca0e7f56b"},
			Username: ptr.To("user2"),
			Origin:   ptr.To("sap.ids"),
		},
	}

	origin := "sap.ids"

	// must find role1 for user1
	role, err := findRole(roles, users, "user1", origin, "space_developer")
	require.NoError(t, err)
	assert.Equal(t, role1.GUID, role.GUID)

	// must error because username must be used
	role, err = findRole(roles, users, "338b0d04-d537-4e4e-8c6f-f09ca0e7f56a", origin, "space_developer")
	require.Error(t, err)
	assert.Nil(t, role)

	// must find role2 for user2
	role, err = findRole(roles, users, "User2", origin, "space_auditor")
	require.NoError(t, err)
	assert.Equal(t, role2.GUID, role.GUID)

	// must error because user3 is not listed in users
	role, err = findRole(roles, users, "user3", origin, "space_developer")
	require.Error(t, err)
	assert.Nil(t, role)

	// must error because role_type mismatch
	role, err = findRole(roles, users, "user2", origin, "space_developer")
	require.Error(t, err)
	assert.Nil(t, role)

}
