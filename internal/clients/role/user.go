package role

import (
	"strings"

	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

// Member identifies a user by name and origin
type Member struct {
	// Username at the identity provider
	Username string `json:"username"`

	// Origin picks the IDP
	Origin string `json:"origin,omitempty"`
}

// Key return a formatted string identifying the Member
func (u *Member) key() string {
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

// toMember converts a username and origin to a Member
func toMember(username string, origin string) *Member {
	return &Member{
		Username: username,
		Origin:   origin,
	}
}

func toMemberKey(u *resource.User) string {
	m := Member{
		Username: u.Username,
		Origin:   u.Origin,
	}
	return m.key()
}
