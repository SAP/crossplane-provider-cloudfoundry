package v1alpha1

import (
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// SpaceReference defines a reference to a Cloud Foundry space.
type SpaceReference struct {
	// (String) The GUID of the Cloud Foundry space. This field is typically populated using references specified in `spaceRef`, `spaceSelector`, or `spaceName`.
	// +crossplane:generate:reference:type=Space
	// +crossplane:generate:reference:extractor=github.com/SAP/crossplane-provider-cloudfoundry/apis/resources.ExternalID()
	Space *string `json:"space,omitempty"`

	// (String) The name of the Cloud Foundry space to lookup the GUID of the space. Use `spaceName` only when the referenced space is not managed by Crossplane.
	// +kubebuilder:validation:Optional
	SpaceName *string `json:"spaceName,omitempty"`

	// (String) The name of the Cloud Foundry organization containing the space.
	// +kubebuilder:validation:Optional
	OrgName *string `json:"orgName,omitempty"`

	// (Attributes) Reference to a `Space` CR to lookup the GUID of the Cloud Foundry space. Preferred if the referenced space is managed by Crossplane.
	// +kubebuilder:validation:Optional
	SpaceRef *xpv2.Reference `json:"spaceRef,omitempty"`

	// (Attributes) Selector for a `Space` CR to lookup the GUID of the Cloud Foundry space. Preferred if the referenced space is managed by Crossplane.
	// +kubebuilder:validation:Optional
	SpaceSelector *xpv2.Selector `json:"spaceSelector,omitempty"`
}

// OrgReference is a struct that represents the reference to an Organization CR.
type OrgReference struct {
	// (String) The GUID of the organization.
	// +crossplane:generate:reference:type=Organization
	// +crossplane:generate:reference:extractor=github.com/SAP/crossplane-provider-cloudfoundry/apis/resources.ExternalID()
	Org *string `json:"org,omitempty"`

	// (String) The name of the Cloud Foundry organization containing the space.
	// +kubebuilder:validation:Optional
	OrgName *string `json:"orgName,omitempty"`

	// (Attributes) Reference to an `Org` CR to retrieve the external GUID of the organization.
	// +kubebuilder:validation:Optional
	OrgRef *xpv2.Reference `json:"orgRef,omitempty"`

	// (Attributes) Selector to an `Org` CR to retrieve the external GUID of the organization.
	// +kubebuilder:validation:Optional
	OrgSelector *xpv2.Selector `json:"orgSelector,omitempty"`
}

// DomainReference defines a reference to a Cloud Foundry Domain.
type DomainReference struct {
	// (String) The GUID of the Cloud Foundry domain. This field is typically populated using references specified in `domainRef`, `domainSelector`, or `domainName`.
	// +crossplane:generate:reference:type=Domain
	// +crossplane:generate:reference:extractor=github.com/SAP/crossplane-provider-cloudfoundry/apis/resources.ExternalID()
	Domain *string `json:"domain,omitempty"`

	// (String) The name of the Cloud Foundry domain to lookup the GUID of the domain. Use `domainName` only when the referenced domain is not managed by Crossplane.
	// +kubebuilder:validation:Optional
	DomainName *string `json:"domainName,omitempty"`

	// (Attributes) Reference to a `domain` CR to lookup the GUID of the Cloud Foundry domain. Preferred if the referenced domain is managed by Crossplane.
	// +kubebuilder:validation:Optional
	DomainRef *xpv2.Reference `json:"domainRef,omitempty"`

	// (Attributes) Selector for a `domain` CR to lookup the GUID of the Cloud Foundry domain. Preferred if the referenced domain is managed by Crossplane.
	// +kubebuilder:validation:Optional
	DomainSelector *xpv2.Selector `json:"domainSelector,omitempty"`
}
