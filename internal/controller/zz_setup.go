// SPDX-FileCopyrightText: 2023 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/upjet/pkg/controller"

	app "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/controller/cloudfoundry/app"
	domain "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/controller/cloudfoundry/domain"
	providerconfig "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/internal/controller/providerconfig"
)

// Setup creates all controllers with the supplied logger and adds them to
// the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		app.Setup,
		domain.Setup,
		providerconfig.Setup,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}
