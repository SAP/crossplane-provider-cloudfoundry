// Package apis contains Kubernetes API for the provider.
package apis

import (
	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes,
		v1alpha1.SchemeBuilder.AddToScheme,
	)
}
