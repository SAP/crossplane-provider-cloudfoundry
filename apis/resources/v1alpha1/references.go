package v1alpha1

import (
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// SpaceReference defines a reference to a Cloud Foundry space
type SpaceReference struct {
	// The `guid` of the Cloud Foundry space. This field is typically populated using references specified in `spaceRef`, `spaceSelector`, or `spaceName`.
	// +crossplane:generate:reference:type=Space
	// +crossplane:generate:reference:extractor=github.com/SAP/crossplane-provider-cloudfoundry/apis/resources.ExternalID()
	Space *string `json:"space,omitempty"`

	// The name of the Cloud Foundry space to lookup the `guid` of the Space. Use `spaceName` only when the reference Space is not managed by Crossplane.
	// +kubebuilder:validation:Optional
	SpaceName *string `json:"spaceName,omitempty"`

	// The name of the Cloud Foundry organization containing the space.
	// +kubebuilder:validation:Optional
	OrgName *string `json:"orgName,omitempty"`

	// Reference to a `Space` CR to lookup the `guid` of the Cloud Foundry space. Preferred if the reference space is managed by Crossplane.
	// +kubebuilder:validation:Optional
	SpaceRef *v1.Reference `json:"spaceRef,omitempty"`

	// Selector for a `Space` CR to lookup the `guid` of the Cloud Foundry space. Preferred if the reference space is managed by Crossplane.
	// +kubebuilder:validation:Optional
	SpaceSelector *v1.Selector `json:"spaceSelector,omitempty"`
}

// OrgReference is a struct that represents the reference to a Organization CR.
type OrgReference struct {
	// (String) The guid of the organization
	// +crossplane:generate:reference:type=Organization
	// +crossplane:generate:reference:extractor=github.com/SAP/crossplane-provider-cloudfoundry/apis/resources.ExternalID()
	Org *string `json:"org,omitempty"`

	// The name of the Cloud Foundry organization containing the space.
	// +kubebuilder:validation:Optional
	OrgName *string `json:"orgName,omitempty"`

	// Reference to an `Org` CR to retrieve the external GUID of the organization.
	// +kubebuilder:validation:Optional
	OrgRef *v1.Reference `json:"orgRef,omitempty"`

	// Selector to an `Org` CR to retrieve the external GUID of the Organization.
	// +kubebuilder:validation:Optional
	OrgSelector *v1.Selector `json:"orgSelector,omitempty"`
}

// DomainReference defines a reference to a Cloud Foundry Domain
type DomainReference struct {
	// The `guid` of the Cloud Foundry domain. This field is typically populated using references specified in `domainRef`, `domainSelector`, or `domainName`.
	// +crossplane:generate:reference:type=Domain
	// +crossplane:generate:reference:extractor=github.com/SAP/crossplane-provider-cloudfoundry/apis/resources.ExternalID()
	Domain *string `json:"domain,omitempty"`

	// The name of the Cloud Foundry domain to lookup the `guid` of the Domain. Use `domainName` only when the referenced Domain is not managed by Crossplane.
	// +kubebuilder:validation:Optional
	DomainName *string `json:"domainName,omitempty"`

	// Reference to a `Domain` CR to lookup the `guid` of the Cloud Foundry domain. Preferred if the reference domain is managed by Crossplane.
	// +kubebuilder:validation:Optional
	DomainRef *v1.Reference `json:"domainRef,omitempty"`

	// Selector for a `Domain` CR to lookup the `guid` of the Cloud Foundry domain. Preferred if the reference domain is managed by Crossplane.
	// +kubebuilder:validation:Optional
	DomainSelector *v1.Selector `json:"domainSelector,omitempty"`
}
