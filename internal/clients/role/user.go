package role

import (
	"strings"

	cfv3 "github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

// Member identifies a user by name and origin
type Member struct {
	// Username at the identity provider
	Username string `json:"username"`

	// Origin picks the IDP
	Origin string `json:"origin,omitempty"`
}

func findRole(roles []*resource.Role, users []*resource.User, username, origin, roleType string) (*resource.Role, error) {
	var userGUID string
	for _, u := range users {
		if strings.EqualFold(u.Username, username) && strings.EqualFold(u.Origin, origin) {
			userGUID = u.GUID
			break
		}
	}

	if userGUID == "" {
		return nil, cfv3.ErrNoResultsReturned
	}

	var noUserRelation resource.ToOneRelationship
	// list of all org users with the specified role type
	for _, ro := range roles {
		if ro.Relationships.User == noUserRelation {
			continue
		}
		if ro.Relationships.User.Data.GUID == userGUID && strings.EqualFold(ro.Type, roleType) {
			return ro, nil
		}

	}
	return nil, cfv3.ErrNoResultsReturned
}
