package adapters

import (
	"os"
	"path/filepath"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"gopkg.in/yaml.v2"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
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
	Org             Org             `yaml:"org"`
	App             App             `yaml:"app"`
	Route           Route           `yaml:"route"`
	ServiceInstance ServiceInstance `yaml:"serviceInstance"`
	SpaceMembers    SpaceMembers    `yaml:"spaceMembers"`
	OrgMembers      OrgMembers      `yaml:"orgMembers"`
}

type Space struct {
	Name               string   `yaml:"name"`
	OrgRef             string   `yaml:"orgRef"`
	ManagementPolicies []string `yaml:"managementPolicies"`
}

type Org struct {
	Name               string   `yaml:"name"`
	ManagementPolicies []string `yaml:"managementPolicies"`
}

type App struct {
	Name               string   `yaml:"name"`
	SpaceRef           string   `yaml:"spaceRef"`
	ManagementPolicies []string `yaml:"managementPolicies"`
}

type Route struct {
	Host               string   `yaml:"host"`
	SpaceRef           string   `yaml:"spaceRef"`
	DomainRef          string   `yaml:"domainRef"`
	ManagementPolicies []string `yaml:"managementPolicies"`
}

type ServiceInstance struct {
	Name               string   `yaml:"name"`
	SpaceRef           string   `yaml:"spaceRef"`
	Type               string   `yaml:"type"`
	ManagementPolicies []string `yaml:"managementPolicies"`
}

type SpaceMembers struct {
	RoleType           string   `yaml:"roleType"`
	SpaceRef           string   `yaml:"spaceRef"`
	ManagementPolicies []string `yaml:"managementPolicies"`
}

type OrgMembers struct {
	RoleType           string   `yaml:"roleType"`
	OrgRef             string   `yaml:"orgRef"`
	ManagementPolicies []string `yaml:"managementPolicies"`
}

// CFResourceFilter implements the ResourceFilter interface
type CFResourceFilter struct {
	Type               string
	Space              *SpaceFilter
	Org                *OrgFilter
	App                *AppFilter
	Route              *RouteFilter
	ServiceInstance    *ServiceInstanceFilter
	SpaceMembers       *SpaceMembersFilter
	OrgMembers         *OrgMembersFilter
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

	if f.Org != nil {
		criteria["name"] = f.Org.Name
	}

	if f.App != nil {
		criteria["name"] = f.App.Name
		criteria["space"] = f.App.SpaceRef
	}

	if f.Route != nil {
		criteria["host"] = f.Route.Host
		criteria["space"] = f.Route.SpaceRef
		criteria["domain"] = f.Route.DomainRef
	}

	if f.ServiceInstance != nil {
		criteria["name"] = f.ServiceInstance.Name
		criteria["space"] = f.ServiceInstance.SpaceRef
		criteria["type"] = f.ServiceInstance.Type
	}

	if f.SpaceMembers != nil {
		criteria["space"] = f.SpaceMembers.SpaceRef
		criteria["role_type"] = f.SpaceMembers.RoleType
	}

	if f.OrgMembers != nil {
		criteria["org"] = f.OrgMembers.OrgRef
		criteria["role_type"] = f.OrgMembers.RoleType
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

type OrgFilter struct {
	Name string
}

type AppFilter struct {
	Name     string
	SpaceRef string
}

type RouteFilter struct {
	Host      string
	SpaceRef  string
	DomainRef string
}

type ServiceInstanceFilter struct {
	Name     string
	SpaceRef string
	Type     string
}

type SpaceMembersFilter struct {
	RoleType string
	SpaceRef string
}

type OrgMembersFilter struct {
	RoleType string
	OrgRef   string
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
					OrgRef: res.Space.OrgRef,
				},
				ManagementPolicies: policies,
			})
		}

		if res.Org.Name != "" {
			var policies []v1.ManagementAction
			for _, policy := range res.Org.ManagementPolicies {
				policies = append(policies, v1.ManagementAction(policy))
			}

			filters = append(filters, &CFResourceFilter{
				Type: v1alpha1.Org_Kind,
				Org: &OrgFilter{
					Name: res.Org.Name,
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
				Type: v1alpha1.App_Kind,
				App: &AppFilter{
					Name:     res.App.Name,
					SpaceRef: res.App.SpaceRef,
				},
				ManagementPolicies: policies,
			})
		}

		if res.Route.Host != "" {
			var policies []v1.ManagementAction
			for _, policy := range res.Route.ManagementPolicies {
				policies = append(policies, v1.ManagementAction(policy))
			}

			filters = append(filters, &CFResourceFilter{
				Type: v1alpha1.RouteKind,
				Route: &RouteFilter{
					Host:      res.Route.Host,
					SpaceRef:  res.Route.SpaceRef,
					DomainRef: res.Route.DomainRef,
				},
				ManagementPolicies: policies,
			})
		}

		if res.ServiceInstance.Name != "" {

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

		if res.SpaceMembers.RoleType != "" {
			var policies []v1.ManagementAction
			for _, policy := range res.SpaceMembers.ManagementPolicies {
				policies = append(policies, v1.ManagementAction(policy))
			}

			filters = append(filters, &CFResourceFilter{
				Type: v1alpha1.SpaceMembersKind,
				SpaceMembers: &SpaceMembersFilter{
					RoleType: res.SpaceMembers.RoleType,
					SpaceRef: res.SpaceMembers.SpaceRef,
				},
				ManagementPolicies: policies,
			})
		}

		if res.OrgMembers.RoleType != "" {
			var policies []v1.ManagementAction
			for _, policy := range res.OrgMembers.ManagementPolicies {
				policies = append(policies, v1.ManagementAction(policy))
			}

			filters = append(filters, &CFResourceFilter{
				Type: v1alpha1.OrgMembersKind,
				OrgMembers: &OrgMembersFilter{
					RoleType: res.OrgMembers.RoleType,
					OrgRef:   res.OrgMembers.OrgRef,
				},
				ManagementPolicies: policies,
			})
		}
	}

	return cfConfig, filters, nil
}
