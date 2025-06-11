package adapters

import (
	"fmt"
	"os"
	"path/filepath"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"gopkg.in/yaml.v2"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/pkg/utils"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/client"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/config"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/resource"
)

// types for the config file
type Config struct {
	Resources         []Resource               `yaml:"resources"`
	ProviderConfigRef client.ProviderConfigRef `yaml:"providerConfigRef"`
}

type Resource struct {
	Space           Space           `yaml:"space"`
	Organization    Organization    `yaml:"org"`
	App             App             `yaml:"app"`
	ServiceInstance ServiceInstance `yaml:"serviceInstance"`
	// add more resources here
}

type Space struct {
	Name               string             `yaml:"name"`
	OrgName            string             `yaml:"orgRef"`
	ManagementPolicies []ManagementPolicy `yaml:"managementPolicies"`
}

type Organization struct {
	Name               string             `yaml:"name"`
	ManagementPolicies []ManagementPolicy `yaml:"managementPolicies"`
}

type App struct {
	Name               string             `yaml:"name"`
	SpaceRef           string             `yaml:"spaceRef"`
	ManagementPolicies []ManagementPolicy `yaml:"managementPolicies"`
}

type ServiceInstance struct {
	Name               string             `yaml:"name"`
	SpaceRef           string             `yaml:"spaceRef"`
	Type               string             `yaml:"type"`
	ManagementPolicies []ManagementPolicy `yaml:"managementPolicies"`
}

type ManagementPolicy string

// CFResourceFilter implements the ResourceFilter interface
type CFResourceFilter struct {
	Type               string
	Space              *SpaceFilter
	Organization       *OrganizationFilter
	App                *AppFilter
	ServiceInstance    *ServiceInstanceFilter
	ManagementPolicies []v1.ManagementAction
}

func (f *CFResourceFilter) GetResourceType() string {
	return f.Type
}

func (f *CFResourceFilter) GetFilterCriteria() map[string]string {
	criteria := make(map[string]string)

	if f.Space != nil {
		criteria["name"] = f.Space.Name
		criteria["org"] = f.Space.OrgRef
	}

	if f.Organization != nil {
		criteria["name"] = f.Organization.Name
	}

	if f.App != nil {
		criteria["name"] = f.App.Name
		criteria["space"] = f.App.SpaceRef
	}

	if f.ServiceInstance != nil {
		criteria["name"] = f.ServiceInstance.Name
		criteria["space"] = f.ServiceInstance.SpaceRef
		criteria["type"] = f.ServiceInstance.Type
	}

	return criteria
}

func (f *CFResourceFilter) GetManagementPolicies() []v1.ManagementAction {
	return f.ManagementPolicies
}

type SpaceFilter struct {
	Name   string
	OrgRef string
}

type OrganizationFilter struct {
	Name string
}

type AppFilter struct {
	Name     string
	SpaceRef string
}

type ServiceInstanceFilter struct {
	Name     string
	SpaceRef string
	Type     string
}

// CFConfig implements the ProviderConfig interface
type CFConfig struct {
	Resources         []Resource
	ProviderConfigRef client.ProviderConfigRef
}

func (c *CFConfig) GetProviderConfigRef() client.ProviderConfigRef {
	return c.ProviderConfigRef
}

func (c *CFConfig) resourceIsValid(resource Resource) bool {
	// check for empty space names
	if resource.Space.Name != "" && (resource.Space.ManagementPolicies == nil || resource.Space.OrgName == "") {
		fmt.Println(resource.Space.Name + "is not a valid space configuration")
		return false
	}
	// check for empty organization names
	if resource.Organization.Name != "" && resource.Organization.ManagementPolicies == nil {
		fmt.Println(resource.Organization.Name + "is not a valid organization configuration")
		return false
	}
	// check for empty app names
	if resource.App.Name != "" && resource.App.ManagementPolicies == nil && resource.App.SpaceRef != "" {
		fmt.Println(resource.App.Name + "is not a valid app configuration")
		return false
	}
	// check for empty service instance names
	if resource.ServiceInstance.Name != "" && (resource.ServiceInstance.ManagementPolicies == nil || resource.ServiceInstance.SpaceRef == "" || resource.ServiceInstance.Type == "") {
		fmt.Println(resource.ServiceInstance.Name + "is not a valid service instance configuration")
		return false
	}
	return true
}

func (c *CFConfig) Validate() bool {
	for _, resource := range c.Resources {
		if !c.resourceIsValid(resource) {
			return false
		}
	}

	// check for empty provider config ref
	if c.ProviderConfigRef.Name == "" || c.ProviderConfigRef.Namespace == "" {
		return false
	}

	return true
}

// CFConfigParser implements the ConfigParser interface
type CFConfigParser struct{}

func (p *CFConfigParser) ParseConfig(configPath string) (config.ProviderConfig, []resource.ResourceFilter, error) {
	// Read config file

	file, err := os.ReadFile(filepath.Clean(configPath))
	if err != nil {
		return nil, nil, err
	}

	// Parse YAML
	var config Config
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		return nil, nil, err
	}

	// Convert to CFConfig
	cfConfig := &CFConfig{
		Resources: config.Resources,
		ProviderConfigRef: client.ProviderConfigRef{
			Name:      config.ProviderConfigRef.Name,
			Namespace: config.ProviderConfigRef.Namespace,
		},
	}

	// Convert to resource filters
	var filters []resource.ResourceFilter
	for _, res := range config.Resources {
		if res.Space.Name != "" {
			var policies []v1.ManagementAction
			for _, policy := range res.Space.ManagementPolicies {
				policies = append(policies, v1.ManagementAction(policy))
			}

			filters = append(filters, &CFResourceFilter{
				Type: v1alpha1.Space_Kind,
				Space: &SpaceFilter{
					Name:   res.Space.Name,
					OrgRef: res.Space.OrgName,
				},
				ManagementPolicies: policies,
			})
		}

		if res.Organization.Name != "" {
			var policies []v1.ManagementAction
			for _, policy := range res.Organization.ManagementPolicies {
				policies = append(policies, v1.ManagementAction(policy))
			}

			filters = append(filters, &CFResourceFilter{
				Type: v1alpha1.Org_Kind,
				Organization: &OrganizationFilter{
					Name: res.Organization.Name,
				},
				ManagementPolicies: policies,
			})
		}

		if res.App.Name != "" {
			utils.PrintLine("add app  ...", res.App.Name, 30)
			var policies []v1.ManagementAction
			for _, policy := range res.App.ManagementPolicies {
				policies = append(policies, v1.ManagementAction(policy))
			}

			filters = append(filters, &CFResourceFilter{
				Type: v1alpha1.App_Kind,
				App: &AppFilter{
					Name:     res.App.Name,
					SpaceRef: res.App.SpaceRef,
				},
				ManagementPolicies: policies,
			})
		}

		if res.ServiceInstance.Name != "" {
			utils.PrintLine("add service instances  ...", res.ServiceInstance.Name, 30)

			var policies []v1.ManagementAction
			for _, policy := range res.ServiceInstance.ManagementPolicies {
				policies = append(policies, v1.ManagementAction(policy))
			}

			filters = append(filters, &CFResourceFilter{
				Type: v1alpha1.ServiceInstance_Kind,
				ServiceInstance: &ServiceInstanceFilter{
					Name:     res.ServiceInstance.Name,
					SpaceRef: res.ServiceInstance.SpaceRef,
					Type:     res.ServiceInstance.Type,
				},
				ManagementPolicies: policies,
			})
		}
	}

	return cfConfig, filters, nil
}
