package domain

import (
	"github.com/crossplane/upjet/pkg/config"
)

// Configure configures individual resources by adding custom ResourceConfigurators.
func Configure(p *config.Provider) {

	p.AddResourceConfigurator("cloudfoundry_domain", func(r *config.Resource) {
		r.UseAsync = true
		r.Version = "v1alpha1"
	})
}
