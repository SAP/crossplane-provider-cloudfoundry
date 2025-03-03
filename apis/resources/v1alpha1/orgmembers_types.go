package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// A OrgRoleType defines the role type of Cloud Foundry Organization.
// +kubebuilder:validation:Enum=Users;Auditors;Managers;BillingManagers
type OrgRoleType string

// StringV3 converts to string accepted by CF V3 API
func (r OrgRoleType) StringV3() string {
	switch r {
	case OrgUser:
		return "organization_user"
	case OrgManager:
		return "organization_manager"
	case OrgAuditor:
		return "organization_auditor"
	case OrgBillingManager:
		return "organization_billing_manager"
	default:
		return ""
	}
}

const (
	// OrgUser specifies the role type organization_user.
	OrgUser OrgRoleType = "Users"

	// OrgManager specifies the role type organization_manager.
	OrgManager OrgRoleType = "Managers"

	// OrgAuditor specifies the role type organization_auditor.
	OrgAuditor OrgRoleType = "Auditors"

	// OrgBillingManager specifies the role type organization_billing_manager. This role is only valid for Cloud Foundry environment deployed with a billing engine.
	OrgBillingManager OrgRoleType = "BillingManagers"
)

// OrgMembersParameters encapsulate role assignments to CloudFoundry Organizations
type OrgMembersParameters struct {
	MemberList `json:",inline"`

	// Org associated guid.
	// +crossplane:generate:reference:type=github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha2.Org
	// +crossplane:generate:reference:extractor=github.com/SAP/crossplane-provider-cloudfoundry/apis/resources.ExternalID()
	// +kubebuilder:validation:Optional
	Org *string `json:"org,omitempty"`

	// Reference to a Org populate org.
	// +kubebuilder:validation:Optional
	OrgRef *v1.Reference `json:"orgRef,omitempty"`

	// Selector for a Org to populate org.
	// +kubebuilder:validation:Optional
	OrgSelector *v1.Selector `json:"orgSelector,omitempty"`

	// Org role type to assign to members; see valid role types https://v3-apidocs.cloudfoundry.org/version/3.127.0/index.html#valid-role-types
	// +kubebuilder:validation:Required

	RoleType OrgRoleType `json:"roleType"`
}

// OrgMembersSpec defines the desired state of OrgMembers
type OrgMembersSpec struct {
	v1.ResourceSpec `json:",inline"`
	ForProvider     OrgMembersParameters `json:"forProvider"`
}

// OrgMembersStatus defines the observed state of OrgMembers.
type OrgMembersStatus struct {
	v1.ResourceStatus `json:",inline"`
	AtProvider        RoleAssignments `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// OrgMembers is the Schema for the OrgMembers API. Provides a Cloud Foundry Org users resource.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,cloudfoundry}
// +kubebuilder:deprecatedversion:warning="v1alpha1/OrgMembers is deprecated. Use v1alpha2/OrgRole instead"
type OrgMembers struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              OrgMembersSpec   `json:"spec"`
	Status            OrgMembersStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OrgMembersList contains a list of OrgMembers
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
