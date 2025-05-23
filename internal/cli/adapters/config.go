package adapters

import (
	"fmt"
	"os"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/client"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/config"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/resource"
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"gopkg.in/yaml.v2"
)

// types for the config file
type Config struct {
	Resources         []Resource            `yaml:"resources"`
	ProviderConfigRef client.ProviderConfigRef `yaml:"providerConfigRef"`
}

type Resource struct{
	Space 			   Space               `yaml:"space"`
	Organization       Organization        `yaml:"org"`
	App 			   App                 `yaml:"app"`
	// add more resources here
}

type Space struct {
	Name                string             `yaml:"name"`
	OrgName             string             `yaml:"orgRef"`
	ManagementPolicies  []ManagementPolicy `yaml:"managementPolicies"`
}

type Organization struct {
	Name                string             `yaml:"name"`
	ManagementPolicies  []ManagementPolicy `yaml:"managementPolicies"`
}

type App struct {
	Name                string             `yaml:"name"`
	SpaceRef            string             `yaml:"spaceRef"`
	ManagementPolicies  []ManagementPolicy `yaml:"managementPolicies"`
}

type ManagementPolicy string

// CFResourceFilter implements the ResourceFilter interface
type CFResourceFilter struct {
	Type              	 string
	Space        		 *SpaceFilter
	Organization         *OrganizationFilter
	App        			 *AppFilter
	ManagementPolicies  []v1.ManagementAction
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

	return criteria
}

func (f *CFResourceFilter) GetManagementPolicies() []v1.ManagementAction {
	return f.ManagementPolicies
}

type SpaceFilter struct {
	Name string
	OrgRef string
}

type OrganizationFilter struct {
	Name   string
}

type AppFilter struct {
	Name   	 string
	SpaceRef string
}

// CFConfig implements the ProviderConfig interface
type CFConfig struct {
	Resources         []Resource
	ProviderConfigRef client.ProviderConfigRef
}

func (c *CFConfig) GetProviderConfigRef() client.ProviderConfigRef {
	return c.ProviderConfigRef
}

func (c *CFConfig) Validate() bool {
	for _, resource := range c.Resources {
		// check for empty space names
		if (resource.Space.Name != "" && (resource.Space.ManagementPolicies == nil || resource.Space.OrgName == "")) {
			fmt.Println(resource.Space.Name + "is not a valid space configuration")
			return false
		}
		// check for empty organization names
		if resource.Organization.Name != "" && resource.Organization.ManagementPolicies == nil{
			fmt.Println(resource.Organization.Name + "is not a valid organization configuration")
			return false
		}
		// check for empty app names
		if resource.App.Name != "" && resource.App.ManagementPolicies == nil && resource.App.SpaceRef != ""{
			fmt.Println(resource.App.Name + "is not a valid app configuration")
			return false
		}
	}

	// check for empty provider config ref
	if c.ProviderConfigRef.Name == "" || c.ProviderConfigRef.Namespace == ""{
		return false
	}
	
	return true
}

// CFConfigParser implements the ConfigParser interface
type CFConfigParser struct{}

func (p *CFConfigParser) ParseConfig(configPath string) (config.ProviderConfig, []resource.ResourceFilter, error) {
	// Read config file
	file, err := os.ReadFile(configPath)
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
				Type: "space",
				Space: &SpaceFilter{
					Name: res.Space.Name,
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
				Type: "organization",
				Organization: &OrganizationFilter{
					Name:   res.Organization.Name,
				},
				ManagementPolicies: policies,
			})
		}

		if res.App.Name != "" {
			var policies []v1.ManagementAction
			for _, policy := range res.App.ManagementPolicies {
				policies = append(policies, v1.ManagementAction(policy))
			}

			filters = append(filters, &CFResourceFilter{
				Type: "app",
				App: &AppFilter{
					Name:   res.App.Name,
					SpaceRef: res.App.SpaceRef,
				},
				ManagementPolicies: policies,
			})
		}
	}

	return cfConfig, filters, nil
}
