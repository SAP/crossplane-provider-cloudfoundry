package space

import (
	"github.com/crossplane/upjet/pkg/config"
)

// Configure configures individual resources by adding custom ResourceConfigurators.
func Configure(p *config.Provider) {
	p.AddResourceConfigurator("cloudfoundry_space_quota", func(r *config.Resource) {
		r.ShortGroup = "cloudfoundry"
		r.UseAsync = true
	})
}
