package adapters

import (
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/provider"
)

type AppConfig struct {
	Name               string             `json:"name"`
	SpaceRef           string             `json:"spaceref"`
	ManagementPolicies managementPolicies `json:"managementpolicies"`
}

var _ provider.ResourceFilter = AppConfig{}

func (ac AppConfig) GetFilterCriteria() map[string]string {
	criteria := make(map[string]string)
	criteria["name"] = ac.Name
	criteria["space"] = ac.SpaceRef
	return criteria
}

func (ac AppConfig) GetManagementPolicies() []v1.ManagementAction {
	return ac.ManagementPolicies.toManagementActions()
}

func (ac AppConfig) GetResourceType() string {
	return v1alpha1.App_Kind
}

type AppConfigs []AppConfig

func (apps AppConfigs) ToResourceFilter() []provider.ResourceFilter {
	result := make([]provider.ResourceFilter, len(apps))
	for i := range apps {
		result[i] = apps[i]
	}
	return result
}
