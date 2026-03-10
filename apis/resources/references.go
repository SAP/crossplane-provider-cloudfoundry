package resources

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/reference"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

// Referenceable return ID for references. All upjet.Observable are referenceable.
type Referenceable interface {
	GetID() string
}

// ExternalID is function to retrieve the external ID of underlying resource.
func ExternalID() reference.ExtractValueFn {
	return func(mg resource.Managed) string {
		o, ok := mg.(Referenceable)
		// If the resource is not referenceable, return zero string
		if !ok {
			return ""
		}
		return o.GetID()
	}
}

// Nameable return name for references.
type Nameable interface {
	GetCloudFoundryName() string
}

// CloudFoundryName is ExtractValueFn to retrieve the resource name in a Cloud Foundry  for a nameable resource.
func CloudFoundryName() reference.ExtractValueFn {
	return func(mg resource.Managed) string {
		o, ok := mg.(Nameable)
		// If the resource is not referenceable, return zero string
		if !ok {
			return ""
		}
		return o.GetCloudFoundryName()
	}
}
