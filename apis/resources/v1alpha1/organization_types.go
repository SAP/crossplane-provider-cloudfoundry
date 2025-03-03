package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// OrganizationObservation Observation for Organization
type OrganizationObservation struct {
	// The GUID of the organization
	ID *string `json:"id,omitempty" tf:"guid,omitempty"`
}

// OrganizationParameters paraleters for a Organization
type OrganizationParameters struct {

	// The name of the Organization in Cloud Foundry
	// +kubebuilder:validation:Optional
	Name *string `json:"name" tf:"name,omitempty"`

	// The external GUID of the org
	// +kubebuilder:validation:Optional
	ID *string `json:"id,omitempty"`
}

// An OrganizationSpec defines the desired state of a Organization.
type OrganizationSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       OrganizationParameters `json:"forProvider,omitempty"`
}

// An OrganizationStatus represents the observed state of a Organization.
type OrganizationStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          OrganizationObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// Organization is the Schema for the Organizations API. Provides a Cloud Foundry Organization resource.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,cloudfoundry}
// +kubebuilder:deprecatedversion:warning="v1alpha1/Organization is deprecated. Use v1alpha2/Org instead"
type Organization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              OrganizationSpec   `json:"spec"`
	Status            OrganizationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OrganizationList contains a list of Organizations
// +kubebuilder:subresource:status
// +kubebuilder:deprecatedversion:warning="Deprecated. Use v1alpha2/Org"
type OrganizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Organization `json:"items"`
}

// Repository type metadata.
var (
	OrganizationKind             = "Organization"
	OrganizationGroupKind        = schema.GroupKind{Group: CRDGroup, Kind: OrganizationKind}.String()
	OrganizationKindAPIVersion   = OrganizationKind + "." + CRDGroupVersion.String()
	OrganizationGroupVersionKind = CRDGroupVersion.WithKind(OrganizationKind)
)

func init() {
	SchemeBuilder.Register(&Organization{}, &OrganizationList{})
}

// GetID returns ID of underlying resource of this App
func (tr *Organization) GetID() string {
	if tr.Status.AtProvider.ID == nil {
		return ""
	}
	return *tr.Status.AtProvider.ID
}
