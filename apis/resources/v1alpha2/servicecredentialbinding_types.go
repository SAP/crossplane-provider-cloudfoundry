/*
Copyright 2023 SAP SE.
*/

package v1alpha2

import (
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ServiceCredentialBindingObservation defines the observed state of ServiceCredentialBinding
type ServiceCredentialBindingObservation struct {
	// The GUID of the service instance.
	ID *string `json:"id,omitempty" tf:"id,omitempty"`

	// LastOperation describes the last operation performed on the service credential binding.
	LastOperation *LastOperation `json:"lastOperation,omitempty"`
}

// ServiceCredentialBindingParameters define the desired state of the forProvider field of ServiceCredentialBinding
type ServiceCredentialBindingParameters struct {
	// The type of the Service Key in Cloud Foundry. Either "key" or "app".
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=key;app
	Type string `json:"type,omitempty"`

	// The name of the Service Key in Cloud Foundry.
	// +kubebuilder:validation:Optional
	Name *string `json:"name,omitempty"`

	// The ID of the Service Instance the key should be associated with.
	// +crossplane:generate:reference:type=github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha2.ServiceInstance
	// +kubebuilder:validation:Optional
	ServiceInstance *string `json:"serviceInstance,omitempty"`

	// Reference to a ManagedServiceInstance to populate serviceInstance.
	// +kubebuilder:validation:Optional
	ServiceInstanceRef *v1.Reference `json:"serviceInstanceRef,omitempty"`

	// Selector for a ManagedServiceInstance to populate serviceInstance.
	// +kubebuilder:validation:Optional
	ServiceInstanceSelector *v1.Selector `json:"serviceInstanceSelector,omitempty"`

	// The ID of an App  that should be bound to. Required if Type is "app".
	// +kubebuilder:validation:Optional
	App *string `json:"app,omitempty"`

	// Arbitrary parameters in the form of stringified JSON object to pass to the service bind handler.
	// +kubebuilder:validation:Optional
	Parameters *runtime.RawExtension `json:"parameters,omitempty"`
}

// ServiceCredentialBindingSpec defines the desired state of ServiceCredentialBinding
type ServiceCredentialBindingSpec struct {
	v1.ResourceSpec `json:",inline"`

	// True to write connectionDetails as single key-value in a secret rather than a map. The key is the metadata.name of the service credential binding CR itself.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	ConnectionDetailsAsJSON bool `json:"connectionDetailsAsJSON,omitempty"`

	ForProvider ServiceCredentialBindingParameters `json:"forProvider"`
}

// ServiceCredentialBindingStatus defines the observed state of ServiceCredentialBinding.
type ServiceCredentialBindingStatus struct {
	v1.ResourceStatus `json:",inline"`
	AtProvider        ServiceCredentialBindingObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// ServiceCredentialBinding is the Schema for the ServiceCredentialBindings API. Provides a Cloud Foundry Service Key.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,cloudfoundry}
type ServiceCredentialBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ServiceCredentialBindingSpec   `json:"spec"`
	Status            ServiceCredentialBindingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ServiceCredentialBindingList contains a list of ServiceCredentialBindings
type ServiceCredentialBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceCredentialBinding `json:"items"`
}

// Repository type metadata.
var (
	ServiceCredentialBindingKind             = "ServiceCredentialBinding"
	ServiceCredentialBindingGroupKind        = schema.GroupKind{Group: CRDGroup, Kind: ServiceCredentialBindingKind}.String()
	ServiceCredentialBindingKindAPIVersion   = ServiceCredentialBindingKind + "." + CRDGroupVersion.String()
	ServiceCredentialBindingGroupVersionKind = CRDGroupVersion.WithKind(ServiceCredentialBindingKind)
)

func init() {
	SchemeBuilder.Register(&ServiceCredentialBinding{}, &ServiceCredentialBindingList{})
}
