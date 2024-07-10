package config

import (
	"github.com/crossplane/crossplane-runtime/pkg/reference"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/upjet/pkg/config"
)

// Referenceable is satisfied by all upjet.Observable.
type Referenceable interface {
	// GetID is in upjet.Observable, i.e., all upjet.Observable is Referenceable.
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

var (
	// ExternalIDFn is the function path that implements `reference.ExtractValueFn``
	ExternalIDFn = "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/config.ExternalID()"

	// OrgType is the package-path that implements the CRD type `Organization`.
	OrgType = "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/organization/v1alpha1.Organization"

	// SpaceType is the package-path that implements the CRD type `Space`
	SpaceType = "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/space/v1alpha1.Space"
)

// CustomReferenceConfigurations configures references to non-terraformed resources, e.g., org, space.
func CustomReferenceConfigurations() config.ResourceOption {
	return func(r *config.Resource) {
		if t, ok := r.TerraformResource.Schema["org"]; ok {
			t.Optional = true

			r.References["org"] = config.Reference{
				Type:      OrgType,
				Extractor: ExternalIDFn,
			}
		}

		if t, ok := r.TerraformResource.Schema["space"]; ok {
			t.Optional = true

			r.References["space"] = config.Reference{
				Type:      SpaceType,
				Extractor: ExternalIDFn,
			}
		}
	}
}
