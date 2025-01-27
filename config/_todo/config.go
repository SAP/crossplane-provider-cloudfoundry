package todo

import "github.com/crossplane/upjet/pkg/config"

// Configure configures individual resources by adding custom ResourceConfigurators.
func Configure(p *config.Provider) {
	//TODO: admin-only, to be completed
	p.AddResourceConfigurator("cloudfoundry_asg", func(r *config.Resource) {
		r.ShortGroup = "cloudfoundry"
		r.Kind = "ASG"
		r.UseAsync = true
	})
	//TODO: admin-only, to be completed
	p.AddResourceConfigurator("cloudfoundry_buildpack", func(r *config.Resource) {
		r.UseAsync = true
	})
	//TODO: admin-only, to be completed
	p.AddResourceConfigurator("cloudfoundry_default_asg", func(r *config.Resource) {
		r.ShortGroup = "cloudfoundry"
		r.Kind = "DefaultASG"
		r.UseAsync = true
	})
	//TODO: admin-only,, to be completed
	p.AddResourceConfigurator("cloudfoundry_evg", func(r *config.Resource) {
		r.ShortGroup = "cloudfoundry"
		r.Kind = "EVG"
		r.UseAsync = true
	})
	//TODO: admin-only,, to be completed
	p.AddResourceConfigurator("cloudfoundry_feature_flags", func(r *config.Resource) {
		r.ShortGroup = "cloudfoundry"
		r.UseAsync = true

	})
	//TODO: admin-only,, to be completed
	p.AddResourceConfigurator("cloudfoundry_isolation_segment", func(r *config.Resource) {
		r.ShortGroup = "cloudfoundry"
		r.UseAsync = true

	})
	//TODO: admin-only, to be completed
	p.AddResourceConfigurator("cloudfoundry_isolation_segment_entitlement", func(r *config.Resource) {
		r.ShortGroup = "cloudfoundry"
		r.UseAsync = true
	})

	//TODO: connect with cf_environment resource in BTP account provider
	p.AddResourceConfigurator("cloudfoundry_org", func(r *config.Resource) {
		r.ShortGroup = "cloudfoundry"
		r.UseAsync = true
	})
	//TODO: managed in BTP account provider
	p.AddResourceConfigurator("cloudfoundry_org_quota", func(r *config.Resource) {
		r.ShortGroup = "cloudfoundry"
		r.UseAsync = true
	})

	//TODO: link to cf environment resource in BTP account provider
	p.AddResourceConfigurator("cloudfoundry_org_users", func(r *config.Resource) {
		r.ShortGroup = "cloudfoundry"
		r.UseAsync = true
	})

	p.AddResourceConfigurator("cloudfoundry_network_policy", func(r *config.Resource) {
		r.ShortGroup = "cloudfoundry"
		r.UseAsync = true
	})

	p.AddResourceConfigurator("cloudfoundry_private_domain_access", func(r *config.Resource) {
		r.ShortGroup = "cloudfoundry"
		r.UseAsync = true
	})
}
