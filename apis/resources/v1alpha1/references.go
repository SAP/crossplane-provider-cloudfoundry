package v1alpha1

import v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

// SpaceRef is a struct that represents the reference to a Space CR.
type SpaceRef struct {
	// Space associated guid.
	// +crossplane:generate:reference:type=Space
	// +crossplane:generate:reference:extractor=github.com/SAP/crossplane-provider-cloudfoundry/apis/resources.ExternalID()
	Space *string `json:"space,omitempty"`

	// Reference to a Space CR to populate space.
	// +kubebuilder:validation:Optional
	SpaceRef *v1.Reference `json:"spaceRef,omitempty"`

	// Selector for a Space CR to populate space.
	// +kubebuilder:validation:Optional
	SpaceSelector *v1.Selector `json:"spaceSelector,omitempty"`
	// Fields relevant  for managed service instances
}

// OrgRef is a struct that represents the reference to a Organization CR.
type OrgRef struct {
	// (String) The guid of the organization
	// +crossplane:generate:reference:type=Organization
	// +crossplane:generate:reference:extractor=github.com/SAP/crossplane-provider-cloudfoundry/apis/resources.ExternalID()
	Org *string `json:"org,omitempty"`

	// Reference to an `Org` CR to retrieve the external GUID of the organization.
	// +kubebuilder:validation:Optional
	OrgRef *v1.Reference `json:"orgRef,omitempty"`

	// Selector to an `Org` CR to retrieve the external GUID of the Organization.
	// +kubebuilder:validation:Optional
	OrgSelector *v1.Selector `json:"orgSelector,omitempty"`
}
