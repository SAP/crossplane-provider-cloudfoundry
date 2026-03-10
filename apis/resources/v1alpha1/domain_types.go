/*
Copyright 2023 SAP SE.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	v2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

type DomainObservation struct {
	// (String) The GUID of the object.
	ID *string `json:"id,omitempty"`

	// (Boolean) Whether the domain is used for internal (container-to-container) traffic, or external (user-to-container) traffic.
	Internal *bool `json:"internal,omitempty"`

	// (String) The name of the domain; must be between 3 ~ 253 characters and follow [RFC 1035](https://tools.ietf.org/html/rfc1035).
	Name *string `json:"name,omitempty"`

	// (String) The organization the domain is scoped to; if set, the domain will only be available in that organization; otherwise, the domain will be globally available.
	Org *string `json:"org,omitempty"`

	// (String) The desired router group guid. Note: creates a TCP domain; cannot be used when `internal` is set to true or domain is scoped to an org.
	RouterGroup *string `json:"routerGroup,omitempty"`

	// (Set of String) Organizations the domain is shared with; if set, the domain will be available in these organizations in addition to the organization the domain is scoped to.
	// +listType=set
	SharedOrgs []*string `json:"sharedOrgs,omitempty"`

	// (Set of String) Available protocols for routes using the domain, currently http and tcp.
	// +listType=set
	SupportedProtocols []*string `json:"supportedProtocols,omitempty"`

	// (String) The date and time when the resource was created in [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) format.
	CreatedAt *string `json:"createdAt,omitempty"`

	// (String) The date and time when the resource was updated in [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) format.
	UpdatedAt *string `json:"updatedAt,omitempty"`

	// (Map of String) The annotations associated with Cloud Foundry resources. Add as described [here](https://docs.cloudfoundry.org/adminguide/metadata.html#-view-metadata-for-an-object).
	// +mapType=granular
	Annotations map[string]*string `json:"annotations,omitempty"`

	// (Map of String) The labels associated with Cloud Foundry resources. Add as described [here](https://docs.cloudfoundry.org/adminguide/metadata.html#-view-metadata-for-an-object).
	// +mapType=granular
	Labels map[string]*string `json:"labels,omitempty"`
}

type DomainParameters struct {
	// (Deprecated) Domain part of full domain name. If specified, the `subDomain` argument needs to be provided and the `name` will be computed. If `name` is provided, `domain` and `subDomain` will be ignored.
	// +kubebuilder:deprecated:warning=The domain field is deprecated and will be removed in a future version. Use the name field instead.
	Domain *string `json:"domain,omitempty" tf:"domain,omitempty"`

	// (Deprecated) Sub-domain part of full domain name. If specified, the `domain` argument needs to be provided and the `name` will be computed. If `name` is provided, `domain` and `subDomain` will be ignored.
	// +kubebuilder:deprecated:warning=The sub_domain field is deprecated and will be removed in a future version. Use the name field instead.
	SubDomain *string `json:"subDomain,omitempty" tf:"sub_domain,omitempty"`

	// (String) The name of the domain; must be between 3 ~ 253 characters and follow [RFC 1035](https://tools.ietf.org/html/rfc1035).
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`

	// (Boolean) Whether the domain is used for internal (container-to-container) traffic, or external (user-to-container) traffic.
	// +kubebuilder:validation:Optional
	Internal *bool `json:"internal,omitempty"`

	// (String) The desired router group guid. Note: creates a TCP domain; cannot be used when `internal` is set to true or domain is scoped to an org.
	// +kubebuilder:validation:Optional
	RouterGroup *string `json:"routerGroup,omitempty"`

	OrgReference `json:",inline"`

	// (Set of String) Organizations the domain is shared with; if set, the domain will be available in these organizations in addition to the organization the domain is scoped to.
	// +kubebuilder:validation:Optional
	// +listType=set
	SharedOrgs []*string `json:"sharedOrgs,omitempty"`

	// (Map of String) The annotations associated with Cloud Foundry resources. Add as described [here](https://docs.cloudfoundry.org/adminguide/metadata.html#-view-metadata-for-an-object).
	// +kubebuilder:validation:Optional
	// +mapType=granular
	Annotations map[string]*string `json:"annotations,omitempty" tf:"annotations,omitempty"`

	// (Map of String) The labels associated with Cloud Foundry resources. Add as described [here](https://docs.cloudfoundry.org/adminguide/metadata.html#-view-metadata-for-an-object).
	// +kubebuilder:validation:Optional
	// +mapType=granular
	Labels map[string]*string `json:"labels,omitempty" tf:"labels,omitempty"`
}

// DomainSpec defines the desired state of Domain
type DomainSpec struct {
	v2.ManagedResourceSpec `json:",inline"`
	ForProvider     DomainParameters `json:"forProvider"`
}

// DomainStatus defines the observed state of Domain.
type DomainStatus struct {
	v1.ResourceStatus `json:",inline"`
	AtProvider        DomainObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// Domain is the Schema for the Domains API. Provides a resource for managing shared or private domains in Cloud Foundry.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,cloudfoundry}
type Domain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DomainSpec   `json:"spec"`
	Status            DomainStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DomainList contains a list of Domains
type DomainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Domain `json:"items"`
}

// Repository type metadata.
var (
	Domain_Kind             = "Domain"
	Domain_GroupKind        = schema.GroupKind{Group: CRDGroup, Kind: Domain_Kind}.String()
	Domain_KindAPIVersion   = Domain_Kind + "." + CRDGroupVersion.String()
	Domain_GroupVersionKind = CRDGroupVersion.WithKind(Domain_Kind)
)

func init() {
	SchemeBuilder.Register(&Domain{}, &DomainList{})
}

// GetID returns the ID of the domain
func (d *Domain) GetID() string {
	if d.Status.AtProvider.ID != nil {
		return *d.Status.AtProvider.ID
	}
	return ""
}
