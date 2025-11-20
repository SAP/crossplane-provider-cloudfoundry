package orgrole

import (
	"context"
	"fmt"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/cache"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/org"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/resources"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/export"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/parsan"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

var (
	roleCache cache.CacheWithGUIDAndName[*role]
	userCache cache.CacheWithGUIDAndName[*user]
	OrgRole   = orgRole{}
)

func init() {
	resources.RegisterKind(OrgRole)
}

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

type role struct {
	*resource.Role
	*resource.User
	*cache.ResourceWithComment
}

func (r *role) GetGUID() string {
	return r.Role.GUID
}

func (r *role) GetName() string {
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

type orgRole struct{}

var _ resources.Kind = orgRole{}

func (om orgRole) Param() configparam.ConfigParam {
	return nil
}

func (om orgRole) KindName() string {
	return "orgrole"
}

func (om orgRole) Export(ctx context.Context, cfClient *client.Client, evHandler export.EventHandler, resolveReferences bool) error {
	orgRoles, _, err := Get(ctx, cfClient)
	if err != nil {
		return err
	}
	if orgRoles.Len() == 0 {
		evHandler.Warn(erratt.New("no orgrole found"))
	} else {
		for _, orgRole := range orgRoles.AllByGUIDs() {
			evHandler.Resource(convertOrgRoleResource(ctx, cfClient, orgRole, evHandler, resolveReferences))
		}
	}
	return nil
}

func Get(ctx context.Context, cfClient *client.Client) (cache.CacheWithGUIDAndName[*role], cache.CacheWithGUID[*user], error) {
	if userCache != nil || roleCache != nil {
		return roleCache, userCache, nil
	}
	orgs, err := org.Get(ctx, cfClient)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	roles, users, err := getAll(ctx, cfClient, orgs.GetGUIDs())
	if err != nil {
		return nil, nil, erratt.Errorf("cannot get roles and users: %w", err)
	}
	roleCache = cache.NewWithGUIDAndName[*role]()
	roleCache.StoreWithGUIDAndName(roles...)
	userCache = cache.NewWithGUIDAndName[*user]()
	userCache.StoreWithGUIDAndName(users...)
	return roleCache, userCache, nil
}

func getAll(ctx context.Context, cfClient *client.Client, orgGuids []string) ([]*role, []*user, error) {
	listOptions := client.NewRoleListOptions()
	listOptions.OrganizationGUIDs.EqualTo(orgGuids...)
	roles, users, err := cfClient.Roles.ListIncludeUsersAll(ctx, listOptions)
	if err != nil {
		return nil, nil, err
	}

	roleResults := make([]*role, len(roles))
	userResults := make([]*user, len(users))
	userGUIDMap := map[string]*resource.User{}

	for i, u := range users {
		userResults[i] = &user{
			User: u,
		}
		userGUIDMap[u.GUID] = u
	}

	for i, r := range roles {
		roleResults[i] = &role{
			ResourceWithComment: &cache.ResourceWithComment{},
			Role:                r,
			User:                userGUIDMap[r.Relationships.User.Data.GUID],
		}
	}

	return roleResults, userResults, nil
}
