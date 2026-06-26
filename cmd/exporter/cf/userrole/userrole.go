package userrole

import (
	"context"
	"fmt"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/org"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/space"

	"github.com/SAP/xp-clifford/erratt"
	"github.com/SAP/xp-clifford/mkcontainer"
	"github.com/SAP/xp-clifford/parsan"
	"github.com/SAP/xp-clifford/yaml"
	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

var (
	orgRoleCache   mkcontainer.TypedContainer[*Role]
	orgUserCache   mkcontainer.TypedContainer[*user]
	spaceRoleCache mkcontainer.TypedContainer[*Role]
	spaceUserCache mkcontainer.TypedContainer[*user]
)

const defaultUserName = "undefined username"

type user struct {
	*resource.User
	*yaml.ResourceWithComment
}

var (
	_ mkcontainer.ItemWithGUID = &user{}
	_ mkcontainer.ItemWithName = &user{}
)

func (u *user) GetGUID() string {
	return u.GUID
}

func (u *user) GetName() string {
	if name := u.Username; name != nil {
		return *name
	}
	return defaultUserName
}

type Role struct {
	*resource.Role
	*resource.User
	*yaml.ResourceWithComment
}

var (
	_ mkcontainer.ItemWithGUID = &Role{}
	_ mkcontainer.ItemWithName = &Role{}
)

func (r *Role) GetGUID() string {
	return r.Role.GUID
}

func (r *Role) GetName() string {
	name := fmt.Sprintf("%s --- %s", *r.Username, r.Type)
	names := parsan.ParseAndSanitize(
		name,
		parsan.RFC1035LowerSubdomain,
	)
	if len(names) == 0 {
		r.AddComment(fmt.Sprintf("error sanitizing name: %s", name))
	} else {
		name = names[0]
	}
	return name
}

func GetOrgRoles(ctx context.Context, cfClient *client.Client) (mkcontainer.TypedContainer[*Role], mkcontainer.TypedContainer[*user], error) {
	if orgUserCache != nil && orgRoleCache != nil {
		return orgRoleCache, orgUserCache, nil
	}
	orgs, err := org.Get(ctx, cfClient)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	roles, users, err := getAll(ctx, cfClient, orgs.GetGUIDs(), nil)
	if err != nil {
		return nil, nil, erratt.Errorf("cannot get roles and users: %w", err)
	}
	orgRoleCache = mkcontainer.NewTyped[*Role]()
	orgUserCache = mkcontainer.NewTyped[*user]()
	orgRoleCache.Store(roles...)
	orgUserCache.Store(users...)
	return orgRoleCache, orgUserCache, nil
}

func GetSpaceRoles(ctx context.Context, cfClient *client.Client) (mkcontainer.TypedContainer[*Role], mkcontainer.TypedContainer[*user], error) {
	if spaceUserCache != nil || spaceRoleCache != nil {
		return spaceRoleCache, spaceUserCache, nil
	}

	orgs, err := org.Get(ctx, cfClient)
	if err != nil {
		return nil, nil, err
	}

	spaces, err := space.Get(ctx, cfClient)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	roles, users, err := getAll(ctx, cfClient, orgs.GetGUIDs(), spaces.GetGUIDs())
	if err != nil {
		return nil, nil, erratt.Errorf("cannot get roles and users: %w", err)
	}
	spaceRoleCache = mkcontainer.NewTyped[*Role]()
	spaceUserCache = mkcontainer.NewTyped[*user]()
	spaceRoleCache.Store(roles...)
	spaceUserCache.Store(users...)
	return spaceRoleCache, spaceUserCache, nil
}

func getAll(ctx context.Context, cfClient *client.Client, orgGuids []string, spaceGuids []string) ([]*Role, []*user, error) {
	listOptions := client.NewRoleListOptions()
	listOptions.OrganizationGUIDs.EqualTo(orgGuids...)
	listOptions.SpaceGUIDs.EqualTo(spaceGuids...)
	roles, users, err := cfClient.Roles.ListIncludeUsersAll(ctx, listOptions)
	if err != nil {
		return nil, nil, err
	}

	roleResults := make([]*Role, len(roles))
	userResults := make([]*user, len(users))
	userGUIDMap := map[string]*resource.User{}

	for i, u := range users {
		userResults[i] = &user{
			User: u,
		}
		userGUIDMap[u.GUID] = u
	}

	for i, r := range roles {
		roleResults[i] = &Role{
			ResourceWithComment: yaml.NewResourceWithComment(nil),
			Role:                r,
			User:                userGUIDMap[r.Relationships.User.Data.GUID],
		}
	}

	return roleResults, userResults, nil
}
