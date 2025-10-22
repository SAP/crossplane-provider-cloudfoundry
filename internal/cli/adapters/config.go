package adapters

import (
	"fmt"
	"os"
	"path/filepath"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"gopkg.in/yaml.v2"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/provider"
)

// types for the config file
type Config struct {
	Resources         []Resource                 `yaml:"resources"`
	ProviderConfigRef provider.ProviderConfigRef `yaml:"providerConfigRef"`
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
	ProviderConfigRef provider.ProviderConfigRef
}

func (c *CFConfig) GetProviderConfigRef() provider.ProviderConfigRef {
	return c.ProviderConfigRef
}

func (c *CFConfig) Validate() bool {
	// Check provider config ref
	if c.ProviderConfigRef.Name == "" || c.ProviderConfigRef.Namespace == "" {
		return false
	}
	// Check if there are any resources to process
	return len(c.Resources) > 0
}

// CFConfigParser implements the ConfigParser interface
type CFConfigParser struct{}

func (p *CFConfigParser) toManagementActions(policies []string) []v1.ManagementAction {
	result := make([]v1.ManagementAction, 0, len(policies))
	for _, policy := range policies {
		result = append(result, v1.ManagementAction(policy))
	}
	return result
}

func (p *CFConfigParser) toSpaceFilter(res Resource) (provider.ResourceFilter, error) {
	if res.Space.OrgRef == "" {
		return nil, fmt.Errorf("space.orgRef is required")
	}

	return &CFResourceFilter{
		Type: v1alpha1.Space_Kind,
		Space: &SpaceFilter{
			Name:   res.Space.Name,
			OrgRef: res.Space.OrgRef,
		},
		ManagementPolicies: p.toManagementActions(res.Space.ManagementPolicies),
	}, nil
}

func (p *CFConfigParser) toOrgFilter(res Resource) (provider.ResourceFilter, error) {
	return &CFResourceFilter{
		Type: v1alpha1.Org_Kind,
		Org: &OrgFilter{
			Name: res.Org.Name,
		},
		ManagementPolicies: p.toManagementActions(res.Org.ManagementPolicies),
	}, nil
}

func (p *CFConfigParser) toAppFilter(res Resource) (provider.ResourceFilter, error) {
	if res.App.SpaceRef == "" {
		return nil, fmt.Errorf("app.spaceRef is required")
	}

	return &CFResourceFilter{
		Type: v1alpha1.App_Kind,
		App: &AppFilter{
			Name:     res.App.Name,
			SpaceRef: res.App.SpaceRef,
		},
		ManagementPolicies: p.toManagementActions(res.App.ManagementPolicies),
	}, nil
}

func (p *CFConfigParser) toServiceInstanceFilter(res Resource) (provider.ResourceFilter, error) {
	if res.ServiceInstance.SpaceRef == "" {
		return nil, fmt.Errorf("serviceInstance.spaceRef is required")
	}
	if res.ServiceInstance.Type == "" {
		return nil, fmt.Errorf("serviceInstance.type is required")
	}

	return &CFResourceFilter{
		Type: v1alpha1.ServiceInstance_Kind,
		ServiceInstance: &ServiceInstanceFilter{
			Name:     res.ServiceInstance.Name,
			SpaceRef: res.ServiceInstance.SpaceRef,
			Type:     res.ServiceInstance.Type,
		},
		ManagementPolicies: p.toManagementActions(res.ServiceInstance.ManagementPolicies),
	}, nil
}

func (p *CFConfigParser) toSpaceMembersFilter(res Resource) (provider.ResourceFilter, error) {
	if res.SpaceMembers.SpaceRef == "" {
		return nil, fmt.Errorf("spaceMembers.spaceRef is required")
	}

	return &CFResourceFilter{
		Type: v1alpha1.SpaceMembersKind,
		SpaceMembers: &SpaceMembersFilter{
			RoleType: res.SpaceMembers.RoleType,
			SpaceRef: res.SpaceMembers.SpaceRef,
		},
		ManagementPolicies: p.toManagementActions(res.SpaceMembers.ManagementPolicies),
	}, nil
}

func (p *CFConfigParser) toOrgMembersFilter(res Resource) (provider.ResourceFilter, error) {
	if res.OrgMembers.OrgRef == "" {
		return nil, fmt.Errorf("orgMembers.orgRef is required")
	}

	return &CFResourceFilter{
		Type: v1alpha1.OrgMembersKind,
		OrgMembers: &OrgMembersFilter{
			RoleType: res.OrgMembers.RoleType,
			OrgRef:   res.OrgMembers.OrgRef,
		},
		ManagementPolicies: p.toManagementActions(res.OrgMembers.ManagementPolicies),
	}, nil
}

func (p *CFConfigParser) toResourceFilter(res Resource) (provider.ResourceFilter, error) {
	switch {
	case res.Space.Name != "":
		return p.toSpaceFilter(res)
	case res.Org.Name != "":
		return p.toOrgFilter(res)
	case res.App.Name != "":
		return p.toAppFilter(res)
	case res.ServiceInstance.Name != "":
		return p.toServiceInstanceFilter(res)
	case res.SpaceMembers.RoleType != "":
		return p.toSpaceMembersFilter(res)
	case res.OrgMembers.RoleType != "":
		return p.toOrgMembersFilter(res)
	default:
		return nil, nil
	}
}

func (p *CFConfigParser) ParseConfig(configPath string) (provider.ProviderConfig, []provider.ResourceFilter, error) {
	file, err := os.ReadFile(filepath.Clean(configPath))
	if err != nil {
		return nil, nil, err
	}

	var config Config
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		return nil, nil, err
	}

	cfConfig := &CFConfig{
		Resources: config.Resources,
		ProviderConfigRef: provider.ProviderConfigRef{
			Name:      config.ProviderConfigRef.Name,
			Namespace: config.ProviderConfigRef.Namespace,
		},
	}

	var filters []provider.ResourceFilter
	for i, res := range config.Resources {
		filter, err := p.toResourceFilter(res)
		if err != nil {
			fmt.Printf("Warning: resource[%d]: %v\n", i, err)
			continue
		}
		if filter != nil {
			filters = append(filters, filter)
		}
	}

	return cfConfig, filters, nil
}
