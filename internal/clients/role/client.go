package role

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"
	"github.com/cloudfoundry/go-cfclient/v3/resource"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/job"
)

// Role is the interface for the role client
type Role interface {
	Get(context.Context, string) (*resource.Role, error)
	Single(context.Context, *client.RoleListOptions) (*resource.Role, error)
	ListIncludeUsersAll(ctx context.Context, opts *client.RoleListOptions) ([]*resource.Role, []*resource.User, error)
	CreateOrganizationRoleWithUsername(context.Context, string, string, resource.OrganizationRoleType, string) (*resource.Role, error)
	CreateSpaceRoleWithUsername(context.Context, string, string, resource.SpaceRoleType, string) (*resource.Role, error)
	Delete(context.Context, string) (string, error)
}

// NewClient returns a new CF client with Role interface
func NewClient(config *config.Config) (Role, job.Job, error) {
	cf, err := client.New(config)
	if err != nil {
		return nil, nil, err
	}
	return cf.Roles, cf.Jobs, nil
}
