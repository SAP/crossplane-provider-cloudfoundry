package adapters

import (
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

type managementPolicies []string

func (mp managementPolicies) toManagementActions() []v1.ManagementAction {
	result := make([]v1.ManagementAction, len(mp))
	for i, policy := range mp {
		result[i] = v1.ManagementAction(policy)
	}
	return result
}
