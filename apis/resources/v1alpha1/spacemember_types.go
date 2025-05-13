package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// SpaceMembersParameters encapsulate role assignments to CloudFoundry Spaces
type SpaceMembersParameters struct {
	SpaceReference `json:",inline"`

	// Space role type to assign to members; see valid role types https://v3-apidocs.cloudfoundry.space/version/3.127.0/index.html#valid-role-types
	// +kubebuilder:validation:Enum=Developer;Auditor;Manager;Supporter;Developers;Auditors;Managers;Supporters
	// +kubebuilder:validation:Required
	RoleType string `json:"roleType"`

	MemberList `json:",inline"`
}

// SpaceMembersSpec defines the desired state of SpaceMembers
type SpaceMembersSpec struct {
	v1.ResourceSpec `json:",inline"`
	ForProvider     SpaceMembersParameters `json:"forProvider"`
}

// SpaceMembersStatus defines the observed state of SpaceMembers.
type SpaceMembersStatus struct {
	v1.ResourceStatus `json:",inline"`
	AtProvider        RoleAssignments `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// SpaceMembers is the Schema for the SpaceMembers API. Provides a Cloud Foundry Space users resource.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,cloudfoundry}
type SpaceMembers struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SpaceMembersSpec   `json:"spec"`
	Status            SpaceMembersStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SpaceMembersList contains a list of SpaceMembers
type SpaceMembersList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SpaceMembers `json:"items"`
}

// Repository type metadata.
var (
	SpaceMembersKind             = "SpaceMembers"
	SpaceMembersGroupKind        = schema.GroupKind{Group: CRDGroup, Kind: SpaceMembersKind}.String()
	SpaceMembersKindAPIVersion   = SpaceMembersKind + "." + CRDGroupVersion.String()
	SpaceMembersGroupVersionKind = CRDGroupVersion.WithKind(SpaceMembersKind)
)

func init() {
	SchemeBuilder.Register(&SpaceMembers{}, &SpaceMembersList{})
}
