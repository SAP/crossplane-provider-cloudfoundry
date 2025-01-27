package org

import "github.com/crossplane/upjet/pkg/config"

const shortGroup = "cloudfoundry"

// Configure configures individual resources by adding custom ResourceConfigurators.
func Configure(p *config.Provider) {
	p.AddResourceConfigurator("cloudfoundry_org", func(r *config.Resource) {
		r.ShortGroup = shortGroup
		r.Version = "v1alpha2"
		r.UseAsync = true
	})

	p.AddResourceConfigurator("cloudfoundry_org_role", func(r *config.Resource) {
		r.ShortGroup = shortGroup
		r.Version = "v1alpha2"
		r.Kind = "OrgRole"
		r.UseAsync = true

		r.References["org"] = config.Reference{
			TerraformName: "cloudfoundry_org"}

	})

	p.AddResourceConfigurator("cloudfoundry_org_quota", func(r *config.Resource) {
		r.ShortGroup = shortGroup
		r.Version = "v1alpha2"
		r.Kind = "OrgQuota"
		r.UseAsync = true

		r.References["orgs"] = config.Reference{
			TerraformName: "cloudfoundry_org"}

	})
}
