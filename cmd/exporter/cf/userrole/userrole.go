package userrole

import (
	"context"
	"fmt"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/cache"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/org"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/space"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/erratt"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/parsan"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

var (
	roleCache cache.CacheWithGUIDAndName[*Role]
	userCache cache.CacheWithGUIDAndName[*user]
)

const defaultUserName = "undefined username"

type user struct {
	*resource.User
	cache.ResourceWithComment
}

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
	*cache.ResourceWithComment
}

func (r *Role) GetGUID() string {
	return r.Role.GUID
}

func (r *Role) GetName() string {
	name := fmt.Sprintf("%s --- %s", *r.User.Username, r.Role.Type)
	names := parsan.ParseAndSanitize(
		name,
		parsan.RFC1035Subdomain,
	)
	if len(names) == 0 {
		r.AddComment(fmt.Sprintf("error sanitizing name: %s", name))
	} else {
		name = names[0]
	}
	return name
}

func GetOrgRoles(ctx context.Context, cfClient *client.Client) (cache.CacheWithGUIDAndName[*Role], cache.CacheWithGUID[*user], error) {
	if userCache != nil || roleCache != nil {
		return roleCache, userCache, nil
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
	roleCache = cache.NewWithGUIDAndName[*Role]()
	roleCache.StoreWithGUIDAndName(roles...)
	userCache = cache.NewWithGUIDAndName[*user]()
	userCache.StoreWithGUIDAndName(users...)
	return roleCache, userCache, nil
}

func GetSpaceRoles(ctx context.Context, cfClient *client.Client) (cache.CacheWithGUIDAndName[*Role], cache.CacheWithGUID[*user], error) {
	if userCache != nil || roleCache != nil {
		return roleCache, userCache, nil
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
	roleCache = cache.NewWithGUIDAndName[*Role]()
	roleCache.StoreWithGUIDAndName(roles...)
	userCache = cache.NewWithGUIDAndName[*user]()
	userCache.StoreWithGUIDAndName(users...)
	return roleCache, userCache, nil
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
			ResourceWithComment: &cache.ResourceWithComment{},
			Role:                r,
			User:                userGUIDMap[r.Relationships.User.Data.GUID],
		}
	}

	return roleResults, userResults, nil
}
