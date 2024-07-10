/*
Copyright 2023 SAP SE
*/

package controller

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/pkg/controller"

	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/controller/organization"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/controller/orgmembers"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/controller/route"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/controller/serviceinstance"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/controller/servicekey"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/controller/space"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/controller/spacemembers"
)

// CustomSetup creates all controllers with the supplied logger and adds them to
// the supplied manager.
func CustomSetup(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		organization.Setup,
		orgmembers.Setup,
		space.Setup,
		spacemembers.Setup,
		route.Setup,
		serviceinstance.Setup,
		servicekey.Setup,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}
