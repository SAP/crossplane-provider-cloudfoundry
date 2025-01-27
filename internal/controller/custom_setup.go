/*
Copyright 2023 SAP SE
*/

package controller

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/pkg/controller"

	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/controller/org"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/controller/orgmembers"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/controller/orgquota"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/controller/orgrole"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/controller/spacemembers"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/controller/spacerole"

	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/controller/route"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/controller/servicecredentialbinding"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/controller/serviceinstance"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/controller/servicekey"
	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/controller/space"
)

// CustomSetup creates all controllers with the supplied logger and adds them to
// the supplied manager.
func CustomSetup(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		org.Setup,
		orgmembers.Setup,
		orgrole.Setup,
		orgquota.Setup,
		space.Setup,
		spacemembers.Setup,
		spacerole.Setup,
		route.Setup,
		serviceinstance.Setup,
		servicekey.Setup,
		servicecredentialbinding.Setup,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}
