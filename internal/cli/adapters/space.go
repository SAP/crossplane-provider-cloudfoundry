package adapters

import (
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/provider"
)

type SpaceConfig struct {
	Name               string             `json:"name"`
	OrgRef             string             `json:"orgref"`
	ManagementPolicies managementPolicies `json:"managementpolicies"`
}

var _ provider.ResourceFilter = SpaceConfig{}

func (sc SpaceConfig) GetFilterCriteria() map[string]string {
	criteria := make(map[string]string)
	criteria["name"] = sc.Name
	criteria["org"] = sc.OrgRef
	return criteria
}

func (sc SpaceConfig) GetManagementPolicies() []v1.ManagementAction {
	return sc.ManagementPolicies.toManagementActions()
}

func (sc SpaceConfig) GetResourceType() string {
	return v1alpha1.Space_Kind
}

type SpaceConfigs []SpaceConfig

func (spaces SpaceConfigs) ToResourceFilter() []provider.ResourceFilter {
	result := make([]provider.ResourceFilter, len(spaces))
	for i := range spaces {
		result[i] = spaces[i]
	}
	return result
}
