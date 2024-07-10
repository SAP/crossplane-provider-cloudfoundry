package route

import (
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/upjet/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/config/observable"
)

// Configure configures individual resources by adding custom ResourceConfigurators.
func Configure(p *config.Provider) {
	p.AddResourceConfigurator("cloudfoundry_route", func(r *config.Resource) {
		r.ShortGroup = "cloudfoundry"
		r.UseAsync = true

		r.References["domain"] = config.Reference{
			Type:              "Domain",
			RefFieldName:      "DomainRef",
			SelectorFieldName: "DomainSelector",
		}

		// add Initializer to allow user to annotate a default domain by name
		r.InitializerFns = append(r.InitializerFns, func(client client.Client) managed.Initializer {
			return observable.NewObserver(client, "domain", &observable.Domain{})
		})

	})

	p.AddResourceConfigurator("cloudfoundry_route_service_binding", func(r *config.Resource) {
		r.Kind = "RouteBinding"
		r.ShortGroup = "cloudfoundry"
		r.UseAsync = true

		r.References["route"] = config.Reference{
			Type:              "Route",
			RefFieldName:      "RouteRef",
			SelectorFieldName: "RouteSelector",
		}
	})
}
