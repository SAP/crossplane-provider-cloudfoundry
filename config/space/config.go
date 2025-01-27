package space

import (
	"github.com/crossplane/upjet/pkg/config"
)

const shortGroup = "cloudfoundry"

// Configure configures individual resources by adding custom ResourceConfigurators.
func Configure(p *config.Provider) {
	p.AddResourceConfigurator("cloudfoundry_space", func(r *config.Resource) {
		r.ShortGroup = shortGroup
		r.Version = "v1alpha2"
		r.UseAsync = true

	})
	p.AddResourceConfigurator("cloudfoundry_space_quota", func(r *config.Resource) {
		r.ShortGroup = shortGroup
		r.Version = "v1alpha2"
		r.UseAsync = true
		r.Kind = "SpaceQuota"

		r.References["space"] = config.Reference{
			TerraformName: "cloudfoundry_space"}
	})

	p.AddResourceConfigurator("cloudfoundry_space_role", func(r *config.Resource) {
		r.ShortGroup = shortGroup
		r.Version = "v1alpha2"
		r.UseAsync = true
		r.Kind = "SpaceRole"

		r.References["space"] = config.Reference{
			TerraformName: "cloudfoundry_space"}

	})

}
