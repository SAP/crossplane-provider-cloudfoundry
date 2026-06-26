/*
Copyright 2023 SAP SE
*/

package controller

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/controller/app"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/controller/domain"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/controller/org"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/controller/orgmembers"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/controller/orgquota"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/controller/orgrole"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/controller/serviceroutebinding"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/controller/spacemembers"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/controller/spacerole"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/controller/route"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/controller/servicecredentialbinding"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/controller/serviceinstance"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/controller/space"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/controller/spacequota"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/controller/providerconfig"
)

// CustomSetup creates all controllers with the supplied logger and adds them to
// the supplied manager.
func CustomSetup(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		providerconfig.Setup,
		app.Setup,
		org.Setup,
		orgrole.Setup,
		orgmembers.Setup,
		orgquota.Setup,
		space.Setup,
		spacerole.Setup,
		spacemembers.Setup,
		route.Setup,
		serviceinstance.Setup,
		servicecredentialbinding.Setup,
		spacequota.Setup,
		domain.Setup,
		serviceroutebinding.Setup,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}
