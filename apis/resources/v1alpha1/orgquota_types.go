/*
Copyright 2023 SAP SE.
*/

// Code generated by upjet. DO NOT EDIT.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

type OrgQuotaInitParameters struct {
	// (Boolean) Determines whether users can provision instances of non-free service plans. Does not control plan visibility. When false, non-free service plans may be visible in the marketplace but instances cannot be provisioned.
	AllowPaidServicePlans *bool `json:"allowPaidServicePlans,omitempty" tf:"allow_paid_service_plans,omitempty"`

	// (Number) Maximum memory per application instance.
	InstanceMemory *float64 `json:"instanceMemory,omitempty" tf:"instance_memory,omitempty"`

	// (String) The name you use to identify the quota or plan in Cloud Foundry.
	Name *string `json:"name,omitempty" tf:"name,omitempty"`

	// (Set of String) Set of Org GUIDs to which this org quota would be assigned.
	// +listType=set
	Orgs []*string `json:"orgs,omitempty" tf:"orgs,omitempty"`

	// (Number) Maximum app instances allowed.
	TotalAppInstances *float64 `json:"totalAppInstances,omitempty" tf:"total_app_instances,omitempty"`

	// (Number) Maximum log rate allowed for all the started processes and running tasks in bytes/second.
	TotalAppLogRateLimit *float64 `json:"totalAppLogRateLimit,omitempty" tf:"total_app_log_rate_limit,omitempty"`

	// (Number) Maximum tasks allowed per app.
	TotalAppTasks *float64 `json:"totalAppTasks,omitempty" tf:"total_app_tasks,omitempty"`

	// (Number) Maximum memory usage allowed.
	TotalMemory *float64 `json:"totalMemory,omitempty" tf:"total_memory,omitempty"`

	// (Number) Maximum number of private domains allowed to be created within the Org.
	TotalPrivateDomains *float64 `json:"totalPrivateDomains,omitempty" tf:"total_private_domains,omitempty"`

	// (Number) Maximum routes with reserved ports.
	TotalRoutePorts *float64 `json:"totalRoutePorts,omitempty" tf:"total_route_ports,omitempty"`

	// (Number) Maximum routes allowed.
	TotalRoutes *float64 `json:"totalRoutes,omitempty" tf:"total_routes,omitempty"`

	// (Number) Maximum service keys allowed.
	TotalServiceKeys *float64 `json:"totalServiceKeys,omitempty" tf:"total_service_keys,omitempty"`

	// (Number) Maximum services allowed.
	TotalServices *float64 `json:"totalServices,omitempty" tf:"total_services,omitempty"`
}

type OrgQuotaObservation struct {
	// (Boolean) Determines whether users can provision instances of non-free service plans. Does not control plan visibility. When false, non-free service plans may be visible in the marketplace but instances cannot be provisioned.
	AllowPaidServicePlans *bool `json:"allowPaidServicePlans,omitempty" tf:"allow_paid_service_plans,omitempty"`

	// (String) The date and time when the resource was created in [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) format.
	CreatedAt *string `json:"createdAt,omitempty" tf:"created_at,omitempty"`

	// (String) The GUID of the object.
	ID *string `json:"id,omitempty" tf:"id,omitempty"`

	// (Number) Maximum memory per application instance.
	InstanceMemory *float64 `json:"instanceMemory,omitempty" tf:"instance_memory,omitempty"`

	// (String) The name you use to identify the quota or plan in Cloud Foundry.
	Name *string `json:"name,omitempty" tf:"name,omitempty"`

	// (Set of String) Set of Org GUIDs to which this org quota would be assigned.
	// +listType=set
	Orgs []*string `json:"orgs,omitempty" tf:"orgs,omitempty"`

	// (Number) Maximum app instances allowed.
	TotalAppInstances *float64 `json:"totalAppInstances,omitempty" tf:"total_app_instances,omitempty"`

	// (Number) Maximum log rate allowed for all the started processes and running tasks in bytes/second.
	TotalAppLogRateLimit *float64 `json:"totalAppLogRateLimit,omitempty" tf:"total_app_log_rate_limit,omitempty"`

	// (Number) Maximum tasks allowed per app.
	TotalAppTasks *float64 `json:"totalAppTasks,omitempty" tf:"total_app_tasks,omitempty"`

	// (Number) Maximum memory usage allowed.
	TotalMemory *float64 `json:"totalMemory,omitempty" tf:"total_memory,omitempty"`

	// (Number) Maximum number of private domains allowed to be created within the Org.
	TotalPrivateDomains *float64 `json:"totalPrivateDomains,omitempty" tf:"total_private_domains,omitempty"`

	// (Number) Maximum routes with reserved ports.
	TotalRoutePorts *float64 `json:"totalRoutePorts,omitempty" tf:"total_route_ports,omitempty"`

	// (Number) Maximum routes allowed.
	TotalRoutes *float64 `json:"totalRoutes,omitempty" tf:"total_routes,omitempty"`

	// (Number) Maximum service keys allowed.
	TotalServiceKeys *float64 `json:"totalServiceKeys,omitempty" tf:"total_service_keys,omitempty"`

	// (Number) Maximum services allowed.
	TotalServices *float64 `json:"totalServices,omitempty" tf:"total_services,omitempty"`

	// (String) The date and time when the resource was updated in [RFC3339](https://www.ietf.org/rfc/rfc3339.txt) format.
	UpdatedAt *string `json:"updatedAt,omitempty" tf:"updated_at,omitempty"`
}

type OrgQuotaParameters struct {
	// (Boolean) Determines whether users can provision instances of non-free service plans. Does not control plan visibility. When false, non-free service plans may be visible in the marketplace but instances cannot be provisioned.
	// +kubebuilder:validation:Optional
	AllowPaidServicePlans *bool `json:"allowPaidServicePlans,omitempty" tf:"allow_paid_service_plans,omitempty"`

	// (Number) Maximum memory per application instance.
	// +kubebuilder:validation:Optional
	InstanceMemory *float64 `json:"instanceMemory,omitempty" tf:"instance_memory,omitempty"`

	// (String) The name you use to identify the quota or plan in Cloud Foundry.
	// +kubebuilder:validation:Optional
	Name *string `json:"name,omitempty" tf:"name,omitempty"`

	// (Set of String) Set of Org GUIDs to which this org quota would be assigned.
	// +kubebuilder:validation:Optional
	// +listType=set
	Orgs []*string `json:"orgs,omitempty" tf:"orgs,omitempty"`

	// (Number) Maximum app instances allowed.
	// +kubebuilder:validation:Optional
	TotalAppInstances *float64 `json:"totalAppInstances,omitempty" tf:"total_app_instances,omitempty"`

	// (Number) Maximum log rate allowed for all the started processes and running tasks in bytes/second.
	// +kubebuilder:validation:Optional
	TotalAppLogRateLimit *float64 `json:"totalAppLogRateLimit,omitempty" tf:"total_app_log_rate_limit,omitempty"`

	// (Number) Maximum tasks allowed per app.
	// +kubebuilder:validation:Optional
	TotalAppTasks *float64 `json:"totalAppTasks,omitempty" tf:"total_app_tasks,omitempty"`

	// (Number) Maximum memory usage allowed.
	// +kubebuilder:validation:Optional
	TotalMemory *float64 `json:"totalMemory,omitempty" tf:"total_memory,omitempty"`

	// (Number) Maximum number of private domains allowed to be created within the Org.
	// +kubebuilder:validation:Optional
	TotalPrivateDomains *float64 `json:"totalPrivateDomains,omitempty" tf:"total_private_domains,omitempty"`

	// (Number) Maximum routes with reserved ports.
	// +kubebuilder:validation:Optional
	TotalRoutePorts *float64 `json:"totalRoutePorts,omitempty" tf:"total_route_ports,omitempty"`

	// (Number) Maximum routes allowed.
	// +kubebuilder:validation:Optional
	TotalRoutes *float64 `json:"totalRoutes,omitempty" tf:"total_routes,omitempty"`

	// (Number) Maximum service keys allowed.
	// +kubebuilder:validation:Optional
	TotalServiceKeys *float64 `json:"totalServiceKeys,omitempty" tf:"total_service_keys,omitempty"`

	// (Number) Maximum services allowed.
	// +kubebuilder:validation:Optional
	TotalServices *float64 `json:"totalServices,omitempty" tf:"total_services,omitempty"`
}

// OrgQuotaSpec defines the desired state of OrgQuota
type OrgQuotaSpec struct {
	v1.ResourceSpec `json:",inline"`
	ForProvider     OrgQuotaParameters `json:"forProvider"`
	// THIS IS A BETA FIELD. It will be honored
	// unless the Management Policies feature flag is disabled.
	// InitProvider holds the same fields as ForProvider, with the exception
	// of Identifier and other resource reference fields. The fields that are
	// in InitProvider are merged into ForProvider when the resource is created.
	// The same fields are also added to the terraform ignore_changes hook, to
	// avoid updating them after creation. This is useful for fields that are
	// required on creation, but we do not desire to update them after creation,
	// for example because of an external controller is managing them, like an
	// autoscaler.
	InitProvider OrgQuotaInitParameters `json:"initProvider,omitempty"`
}

// OrgQuotaStatus defines the observed state of OrgQuota.
type OrgQuotaStatus struct {
	v1.ResourceStatus `json:",inline"`
	AtProvider        OrgQuotaObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// OrgQuota is the Schema for the OrgQuotas API. Provides a Cloud Foundry resource to manage org quota definitions.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,cloudfoundry}
type OrgQuota struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +kubebuilder:validation:XValidation:rule="!('*' in self.managementPolicies || 'Create' in self.managementPolicies || 'Update' in self.managementPolicies) || has(self.forProvider.allowPaidServicePlans) || (has(self.initProvider) && has(self.initProvider.allowPaidServicePlans))",message="spec.forProvider.allowPaidServicePlans is a required parameter"
	// +kubebuilder:validation:XValidation:rule="!('*' in self.managementPolicies || 'Create' in self.managementPolicies || 'Update' in self.managementPolicies) || has(self.forProvider.name) || (has(self.initProvider) && has(self.initProvider.name))",message="spec.forProvider.name is a required parameter"
	Spec   OrgQuotaSpec   `json:"spec"`
	Status OrgQuotaStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OrgQuotaList contains a list of OrgQuotas
type OrgQuotaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OrgQuota `json:"items"`
}

// Repository type metadata.
var (
	OrgQuota_Kind             = "OrgQuota"
	OrgQuota_GroupKind        = schema.GroupKind{Group: CRDGroup, Kind: OrgQuota_Kind}.String()
	OrgQuota_KindAPIVersion   = OrgQuota_Kind + "." + CRDGroupVersion.String()
	OrgQuota_GroupVersionKind = CRDGroupVersion.WithKind(OrgQuota_Kind)
)

func init() {
	SchemeBuilder.Register(&OrgQuota{}, &OrgQuotaList{})
}
