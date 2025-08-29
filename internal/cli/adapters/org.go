package adapters

import (
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/provider"
)

type OrgConfig struct {
	Name               string             `json:"name"`
	ManagementPolicies managementPolicies `json:"managementpolicies"`
}

var _ provider.ResourceFilter = OrgConfig{}

func (oc OrgConfig) GetFilterCriteria() map[string]string {
	criteria := make(map[string]string)
	criteria["name"] = oc.Name
	return criteria
}

func (oc OrgConfig) GetManagementPolicies() []v1.ManagementAction {
	return oc.ManagementPolicies.toManagementActions()
}

func (oc OrgConfig) GetResourceType() string {
	return v1alpha1.Org_Kind
}

type OrgConfigs []OrgConfig

func (orgs OrgConfigs) ToResourceFilter() []provider.ResourceFilter {
	result := make([]provider.ResourceFilter, len(orgs))
	for i := range orgs {
		result[i] = orgs[i]
	}
	return result
}
