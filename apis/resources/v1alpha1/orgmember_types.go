package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

type OrgMembersParameters struct {
	MemberList `json:",inline"`

	OrgReference `json:",inline"`

	// (String) Org role type to assign to members; see valid role types https://v3-apidocs.cloudfoundry.org/version/3.127.0/index.html#valid-role-types
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=User;Auditor;Manager;BillingManager;Users;Auditors;Managers;BillingManagers
	RoleType string `json:"roleType"`
}

type OrgMembersSpec struct {
	v1.ResourceSpec `json:",inline"`
	ForProvider     OrgMembersParameters `json:"forProvider"`
}

type OrgMembersStatus struct {
	v1.ResourceStatus `json:",inline"`
	// (Attributes) The assigned roles for the organization members.
	AtProvider RoleAssignments `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// OrgMembers is the Schema for the OrgMembers API. Provides a Cloud Foundry Org users resource.
//
// External-Name Configuration:
//   - Follows Standard: no (uses compound key <org-guid>/<role-type>, not a single GUID)
//   - Format: <org-guid>/<role-type>
//   - How to find:
//     - UI: BTP Cockpit → Subaccounts → [Select Subaccount] → Cloud Foundry → Organization → Org ID + Settings → Org Members
//     - CLI: Use CF CLI: `cf org <ORG_NAME> --guid` combined with role type
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,cloudfoundry}
// +kubebuilder:validation:XValidation:rule="self.spec.managementPolicies == ['Observe'] || has(self.spec.forProvider.roleType)",message="roleType is required"
// +kubebuilder:validation:XValidation:rule="self.spec.managementPolicies == ['Observe'] || (has(self.spec.forProvider.orgName) || has(self.spec.forProvider.orgRef) || has(self.spec.forProvider.orgSelector))",message="OrgReference is required: exactly one of orgName, orgRef, or orgSelector must be set"
// +kubebuilder:validation:XValidation:rule="self.spec.managementPolicies == ['Observe'] || (has(self.spec.forProvider.members) && self.spec.forProvider.members.size() >= 1)",message="Members validation: at least one member must be set"
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
