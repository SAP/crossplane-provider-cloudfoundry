package serviceinstance

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

const (
	errMissingServicePlan = "managed resource service instance requires a service plan"
)

type ServicePlan interface {
	Get(ctx context.Context, guid string) (*resource.ServicePlan, error)
	Single(ctx context.Context, opts *client.ServicePlanListOptions) (*resource.ServicePlan, error)
}

type ServicePlanResolver interface {
	ServicePlan
}

// We either populate/update the service plan ID with the external resource GUID
// based on the specified offering and plan or we use the provided ID directly.
func (c *Client) ResolveServicePlan(ctx context.Context, kube k8s.Client, cr *v1alpha1.ServiceInstance) error {
	if cr.Spec.ForProvider.ServicePlan.Offering != nil && cr.Spec.ForProvider.ServicePlan.Plan != nil {
		return c.resolvePlanID(ctx, kube, cr)
	}

	if cr.Spec.ForProvider.ServicePlan.ID != nil {
		return c.validatePlanID(ctx, cr)
	}

	return errors.New(errMissingServicePlan)
}

// Populate/Update service plan ID based on offering and plan
func (c *Client) resolvePlanID(ctx context.Context, kube k8s.Client, cr *v1alpha1.ServiceInstance) error {
	opt := client.NewServicePlanListOptions()
	opt.ServiceOfferingNames.EqualTo(*cr.Spec.ForProvider.ServicePlan.Offering)
	opt.Names.EqualTo(*cr.Spec.ForProvider.ServicePlan.Plan)

	res, err := c.ServicePlanResolver.Single(ctx, opt)
	if err != nil {
		return errors.Wrapf(err, "cannot initialize service plan using serviceName/servicePlanName: %s:%s", *cr.Spec.ForProvider.ServicePlan.Offering, *cr.Spec.ForProvider.ServicePlan.Plan)
	}

	cr.Spec.ForProvider.ServicePlan.ID = &res.GUID
	return kube.Update(ctx, cr)
}

// Verify whether service plan ID is valid
func (c *Client) validatePlanID(ctx context.Context, cr *v1alpha1.ServiceInstance) error {
	_, err := c.ServicePlanResolver.Get(ctx, *cr.Spec.ForProvider.ServicePlan.ID)
	if err != nil {
		return errors.Wrapf(err, "cannot initialize service plan using ID: %s", *cr.Spec.ForProvider.ServicePlan.ID)
	}
	return nil
}
