package observable

import (
	"context"
	"encoding/json"

	cfclient "github.com/cloudfoundry-community/go-cfclient/v3/client"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/pkg/errors"
)

// ServicePlan is an observable external data source which is not managed in this provider
type ServicePlan struct {
	ID       string `json:"id"`
	Instance struct {
		Offering string `json:"offering"`
		Plan     string `json:"plan"`
	} `json:"service_plan"`
}

// Instantiate extracts ServicePlan from crossplane.io/external-data
func (s *ServicePlan) Instantiate(mg resource.Managed, key string) bool {
	sp, ok := mg.GetAnnotations()[key]
	if !ok {
		return false
	}
	if err := json.Unmarshal([]byte(sp), s); err != nil {
		return false
	}
	return s.Instance.Offering != "" && s.Instance.Plan != ""
}

// Read populate the Service Plan instance
func (s *ServicePlan) Read(ctx context.Context, connectFn ConnectFn) error {
	c, err := connectFn(ctx)
	if err != nil {
		return errors.Wrap(err, "cannot connect to external data source")
	}
	sp, err := c.ServicePlans.Single(ctx,
		&cfclient.ServicePlanListOptions{
			Names:                cfclient.Filter{Values: []string{s.Instance.Plan}},
			ServiceOfferingNames: cfclient.Filter{Values: []string{s.Instance.Offering}},
		})
	if err != nil {
		return errors.Wrap(err, "cannot observe the specified service plan")
	}
	s.ID = sp.GUID
	return nil
}

// GetID returns the observed ID
func (s *ServicePlan) GetID() string {
	return s.ID
}
