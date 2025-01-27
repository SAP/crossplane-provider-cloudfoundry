package app

import (
	"github.com/crossplane/upjet/pkg/config"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// Configure configures individual resources by adding custom ResourceConfigurators.
func Configure(p *config.Provider) {
	p.AddResourceConfigurator("cloudfoundry_app", func(r *config.Resource) {
		r.ShortGroup = "cloudfoundry"
		r.Version = "v1alpha1"
		r.UseAsync = true

		// Configure the dockerVersion field as a string

		// If needed, explicitly define the schema for docker_credentials
		r.TerraformResource.Schema["docker_credentials"] = &schema.Schema{
			Type:      schema.TypeMap,
			Optional:  true,
			Sensitive: true,
		}

		// Routes configuration
		r.TerraformResource.Schema["routes"] = &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"route": {
						Type:     schema.TypeString,
						Required: true,
					},
					"protocol": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			},
		}
		// Add reference to `Route` CR
		r.References["routes.route"] = config.Reference{
			Type: "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/resources/v1alpha1.Route",
		}

		// Service Bindings configuration
		r.TerraformResource.Schema["service_bindings"] = &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"service_instance": {
						Type:     schema.TypeString,
						Required: true,
					},
					"params": {
						Type:         schema.TypeString,
						Optional:     true,
						ValidateFunc: validation.StringIsJSON,
					},
				},
			},
		}
		// Add reference to `ServiceInstance` CR
		r.References["service_bindings.service_instance"] = config.Reference{
			Type: "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/resources/v1alpha2.ServiceInstance",
		}

		// Sidecars configuration
		r.TerraformResource.Schema["sidecars"] = &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"name": {
						Type:     schema.TypeString,
						Required: true,
					},
					"command": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"memory": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"process_types": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem: &schema.Schema{
							Type: schema.TypeString,
						},
					},
				},
			},
		}

		r.TerraformResource.Schema["processes"] = &schema.Schema{
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"type": {
						Type:     schema.TypeString,
						Required: true,
					},
					"command": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"disk_quota": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"health_check_http_endpoint": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"health_check_interval": {
						Type:     schema.TypeInt,
						Optional: true,
					},
					"health_check_invocation_timeout": {
						Type:     schema.TypeInt,
						Optional: true,
					},
					"health_check_type": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"instances": {
						Type:     schema.TypeInt,
						Optional: true,
						Default:  1,
					},
					"log_rate_limit_per_second": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"memory": {
						Type:     schema.TypeString,
						Optional: true,
						Computed: true,
					},
					"readiness_health_check_http_endpoint": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"readiness_health_check_interval": {
						Type:     schema.TypeInt,
						Optional: true,
					},
					"readiness_health_check_invocation_timeout": {
						Type:     schema.TypeInt,
						Optional: true,
					},
					"readiness_health_check_type": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"timeout": {
						Type:     schema.TypeInt,
						Optional: true,
					},
				},
			},
		}
	})
}
