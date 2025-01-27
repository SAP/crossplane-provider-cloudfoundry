package resources

import (
	"github.com/crossplane/crossplane-runtime/pkg/reference"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
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
