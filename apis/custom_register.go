// Package apis contains Kubernetes API for the provider.
package apis

import (
	members "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/members/v1alpha1"
	org "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/organization/v1alpha1"
	route "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/route/v1alpha1"
	service "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/service/v1alpha1"
	servicekey "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/servicekey/v1alpha1"
	space "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/space/v1alpha1"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes,
		org.SchemeBuilder.AddToScheme,
		space.SchemeBuilder.AddToScheme,
		members.SchemeBuilder.AddToScheme,
		route.SchemeBuilder.AddToScheme,
		servicekey.SchemeBuilder.AddToScheme,
		service.SchemeBuilder.AddToScheme,
	)
}
