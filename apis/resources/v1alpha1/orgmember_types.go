package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	v2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

type OrgMembersParameters struct {
	MemberList `json:",inline"`

	OrgReference `json:",inline"`

	// (String) Org role type to assign to members; see valid role types https://v3-apidocs.cloudfoundry.org/version/3.127.0/index.html#valid-role-types
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=User;Auditor;Manager;BillingManager;Users;Auditors;Managers;BillingManagers
	RoleType string `json:"roleType"`
}

type OrgMembersSpec struct {
	v2.ManagedResourceSpec `json:",inline"`
	ForProvider     OrgMembersParameters `json:"forProvider"`
}

type OrgMembersStatus struct {
	v1.ResourceStatus `json:",inline"`
	// (Attributes) The assigned roles for the organization members.
	AtProvider RoleAssignments `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// OrgMembers is the Schema for the OrgMembers API. Provides a Cloud Foundry Org users resource.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,cloudfoundry}
type OrgMembers struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              OrgMembersSpec   `json:"spec"`
	Status            OrgMembersStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OrgMembersList contains a list of OrgMembers.
type OrgMembersList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OrgMembers `json:"items"`
}

// Repository type metadata.
var (
	OrgMembersKind             = "OrgMembers"
	OrgMembersGroupKind        = schema.GroupKind{Group: CRDGroup, Kind: OrgMembersKind}.String()
	OrgMembersKindAPIVersion   = OrgMembersKind + "." + CRDGroupVersion.String()
	OrgMembersGroupVersionKind = CRDGroupVersion.WithKind(OrgMembersKind)
)

func init() {
	SchemeBuilder.Register(&OrgMembers{}, &OrgMembersList{})
}
