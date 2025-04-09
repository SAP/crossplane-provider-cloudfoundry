package spacequota

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

type SpaceQuotaClient interface {
	Get(ctx context.Context, guid string) (*resource.SpaceQuota, error)
	Create(ctx context.Context, r *resource.SpaceQuotaCreateOrUpdate) (*resource.SpaceQuota, error)
	Update(ctx context.Context, guid string, r *resource.SpaceQuotaCreateOrUpdate) (*resource.SpaceQuota, error)
	Apply(ctx context.Context, guid string, spaceGUIDs []string) ([]string, error)
	Remove(ctx context.Context, guid, spaceGUID string) error
	Delete(ctx context.Context, guid string) (string, error)
}

func NewClient(cf *client.Client) SpaceQuotaClient {
	return cf.SpaceQuotas
}
