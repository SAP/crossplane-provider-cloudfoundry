package service

import (
	"github.com/crossplane/upjet/pkg/config"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const shortGroup = "cloudfoundry"

// Configure configures individual resources by adding custom ResourceConfigurators.
func Configure(p *config.Provider) {
	p.AddResourceConfigurator("cloudfoundry_service_instance", func(r *config.Resource) {
		r.ShortGroup = shortGroup
		r.Version = "v1alpha2"
		r.UseAsync = true
		r.Kind = "ServiceInstance"

		// Last_operation configuration
		r.TerraformResource.Schema["last_operation"] = &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"created_at": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"description": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"state": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"type": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem: &schema.Schema{
							Type: schema.TypeString,
						},
					},
					"updated_at": {
						Type:     schema.TypeString,
						Computed: true,
					},
				},
			},
		}
		// maintenance_info configuration
		r.TerraformResource.Schema["maintenance_info"] = &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"version": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"description": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			},
		}
		// timeouts configuration
		r.TerraformResource.Schema["timeouts"] = &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"create": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"delete": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"update": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			},
		}

	})
}
