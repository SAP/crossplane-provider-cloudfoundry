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

// Config holds provider settings that controller.Options does not cover.
type Config struct {
	// SCBMaxCreateAttempts is the ServiceCredentialBinding create-attempt limit.
	SCBMaxCreateAttempts int
}

// CustomSetup creates all controllers with the supplied logger and adds them to
// the supplied manager.
func CustomSetup(mgr ctrl.Manager, o controller.Options, cfg Config) error {
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
		servicecredentialbinding.SetupWithMaxCreateAttempts(cfg.SCBMaxCreateAttempts),
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
