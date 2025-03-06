package role

import (
	"context"
	"errors"
	"time"

	"k8s.io/utils/ptr"

	cfv3 "github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha2"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
)

const ErrSpaceNotSpecified = "Space is not specified"

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

// GetSpaceRole returns the role of a user in a space by guid or by matching the spec
func GetSpaceRole(ctx context.Context, client Role, guid string, spec v1alpha2.SpaceRoleParameters) (*resource.Role, error) {
	if clients.IsValidGUID(guid) {
		return client.Get(ctx, guid)
	}
	return findSpaceRole(ctx, client, spec)
}

// searchSpaceRole returns the role of a user in a space if the role matches the spec
func findSpaceRole(ctx context.Context, client Role, spec v1alpha2.SpaceRoleParameters) (*resource.Role, error) {

	opt, err := newSpaceRoleListOptions(spec)
	if err != nil {
		return nil, err
	}

	roles, users, err := client.ListIncludeUsersAll(ctx, opt)
	if err != nil {
		return nil, err
	}

	return findRole(roles, users, spec.Username,
		ptr.Deref(spec.Origin, "sap.ids"),
		SpaceRoleType(spec.Type).String(),
	)
}

// newSpaceRoleListOptions returns a list options for the given SpaceRoleParameters
func newSpaceRoleListOptions(spec v1alpha2.SpaceRoleParameters) (*cfv3.RoleListOptions, error) {
	opts := cfv3.NewRoleListOptions()
	opts.WithSpaceRoleType(SpaceRoleType(spec.Type))

	// Space (guid) is required
	if spec.Space == nil {
		return nil, errors.New(ErrSpaceNotSpecified)
	}
	opts.SpaceGUIDs.EqualTo(*spec.Space)

	return opts, nil
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
