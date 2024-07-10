package observable

import (
	"context"
	"encoding/json"

	cfclient "github.com/cloudfoundry-community/go-cfclient/v3/client"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/pkg/errors"
)

// Domain is an observable external data source which is not managed by this provider
type Domain struct {
	ID   string `json:"id"`
	Name string `json:"default_domain"`
}

// Instantiate extracts Domain from crossplane.io/external-data
func (s *Domain) Instantiate(mg resource.Managed, key string) bool {
	sp, ok := mg.GetAnnotations()[key]
	if !ok {
		return false
	}
	if err := json.Unmarshal([]byte(sp), s); err != nil {
		return false
	}
	return s.Name != ""
}

// Read populate the Domain instance
func (s *Domain) Read(ctx context.Context, connectFn ConnectFn) error {
	c, err := connectFn(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot connect to external data source")
	}
	sp, err := c.Domains.Single(ctx,
		&cfclient.DomainListOptions{
			Names: cfclient.Filter{Values: []string{s.Name}},
		})
	if err != nil {
		return errors.Wrap(err, "cannot observe the specified domain")
	}
	s.ID = sp.GUID
	return nil
}

// GetID returns the observed ID
func (s *Domain) GetID() string {
	return s.ID
}
