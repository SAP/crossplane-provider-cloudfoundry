package domain

import (
	"github.com/crossplane/upjet/pkg/config"
)

// Configure configures individual resources by adding custom ResourceConfigurators.
func Configure(p *config.Provider) {

	p.AddResourceConfigurator("cloudfoundry_domain", func(r *config.Resource) {
		r.ShortGroup = "cloudfoundry"
		r.UseAsync = true
	})

	p.AddResourceConfigurator("cloudfoundry_private_domain_access", func(r *config.Resource) {
		r.Kind = "PrivateDomainAccess"
		r.ShortGroup = "cloudfoundry"
		r.UseAsync = true

		// field `domain` references of a `Domain` MR
		r.References["domain"] = config.Reference{
			Type:              "Domain",
			RefFieldName:      "DomainRef",
			SelectorFieldName: "DomainSelector",
		}
	})
}
