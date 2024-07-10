package app

import (
	"github.com/crossplane/upjet/pkg/config"
)

// Configure configures individual resources by adding custom ResourceConfigurators.
func Configure(p *config.Provider) {
	p.AddResourceConfigurator("cloudfoundry_app", func(r *config.Resource) {
		r.ShortGroup = "cloudfoundry"
		r.UseAsync = true

		// Add reference to `Route` CR
		r.References["routes.route"] = config.Reference{
			Type: "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/route/v1alpha1.Route",
		}

		// Add reference to `ServiceInstance` CR
		r.References["service_binding.service_instance"] = config.Reference{
			Type: "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/service/v1alpha1.ServiceInstance",
		}
	})
}
