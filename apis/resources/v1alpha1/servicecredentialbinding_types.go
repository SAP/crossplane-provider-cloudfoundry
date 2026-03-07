/*
Copyright 2023 SAP SE.
*/

package v1alpha1

import (
	v1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	v2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ServiceCredentialBindingObservation struct {
	SCBResource `json:",inline"`
	// (Attributes) The details of the last operation performed on the service credential binding.
	LastOperation *LastOperation `json:"lastOperation,omitempty"`

	// If the binding is rotated, `retiredBindings` stores resources that have been rotated out but are still transitionally retained due to `rotation.ttl` setting
	// +kubebuilder:validation:Optional
	RetiredKeys []*SCBResource `json:"retiredKeys,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="!(has(self.type) && self.type == 'app') || !has(self.rotation)",message="rotation cannot be enabled when type is app"
// +kubebuilder:validation:XValidation:rule="!(has(self.type) && self.type == 'key') || has(self.name)",message="name is required when type is key"
// +kubebuilder:validation:XValidation:rule="!(has(self.type) && self.type == 'app') || has(self.app) || has(self.appRef) || has(self.appSelector)",message="app, appRef, or appSelector is required when type is app"
type ServiceCredentialBindingParameters struct {
	// (String) The type of the service credential binding in Cloud Foundry. Either "key" or "app".
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=key;app
	// +kubebuilder:default=key
	Type string `json:"type,omitempty"`

	// (String) The name of the service credential binding in Cloud Foundry. Required if `type` is "key".
	// +kubebuilder:validation:Optional
	Name *string `json:"name,omitempty"`

	// (String) The ID of the service instance the binding should be associated with.
	// +crossplane:generate:reference:type=github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1.ServiceInstance
	// +kubebuilder:validation:Optional
	ServiceInstance *string `json:"serviceInstance,omitempty"`

	// (Attributes) Reference to a managed service instance to populate `serviceInstance`.
	// +kubebuilder:validation:Optional
	ServiceInstanceRef *v1.NamespacedReference `json:"serviceInstanceRef,omitempty"`

	// (Attributes) Selector for a managed service instance to populate `serviceInstance`.
	// +kubebuilder:validation:Optional
	ServiceInstanceSelector *v1.NamespacedSelector `json:"serviceInstanceSelector,omitempty"`

	// (String) The ID of an app that should be bound to. Required if `type` is "app".
	// +crossplane:generate:reference:type=App
	// +kubebuilder:validation:Optional
	App *string `json:"app,omitempty"`

	// (Attributes) Reference to an app CR to populate `app`.
	// +kubebuilder:validation:Optional
	AppRef *v1.NamespacedReference `json:"appRef,omitempty"`

	// (Attributes) Selector for an app CR to populate `app`.
	// +kubebuilder:validation:Optional
	AppSelector *v1.NamespacedSelector `json:"appSelector,omitempty"`

	// (Attributes) An optional JSON object to pass `parameters` to the service broker.
	// +kubebuilder:validation:Optional
	Parameters *runtime.RawExtension `json:"parameters,omitempty"`

	// (Attributes) Use a reference to a secret to pass `parameters` to the service broker. Ignored if `parameters` is set.
	// +kubebuilder:validation:Optional
	ParametersSecretRef *v1.SecretReference `json:"paramsSecretRef,omitempty"`

	// (Boolean, Deprecated) True to write `connectionDetails` as a single key-value in a secret rather than a map. The key is the metadata.name of the service credential binding CR itself. This is deprecated in favor of the `spec.connectionDetailsAsJSON` field.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	ConnectionDetailsAsJSON bool `json:"connectionDetailsAsJSON,omitempty"`

	// Rotation defines the parameters for rotating the service credential binding.
	// +kubebuilder:validation:Optional
	Rotation *RotationParameters `json:"rotation,omitempty"`
}

type ServiceCredentialBindingSpec struct {
	v2.ManagedResourceSpec `json:",inline"`

	// (Boolean) True to write `connectionDetails` as a single key-value in a secret rather than a map. The key is the metadata.name of the service credential binding CR itself.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	ConnectionDetailsAsJSON bool `json:"connectionDetailsAsJSON,omitempty"`

	ForProvider ServiceCredentialBindingParameters `json:"forProvider"`
}

// +kubebuilder:validation:XValidation:rule="!has(self.ttl) || (has(self.frequency) && duration(self.ttl) >= duration(self.frequency))",message="ttl must be greater than or equal to frequency"
type RotationParameters struct {
	// Frequency defines how often the active key should be rotated.
	// +kubebuilder:validation:Required
	Frequency *metav1.Duration `json:"frequency"`

	// TTL (Time-To-Live) defines the total time a credential is valid for before it is deleted.
	// Must be >= frequency
	// +kubebuilder:validation:Optional
	TTL *metav1.Duration `json:"ttl,omitempty"`
}
type ServiceCredentialBindingStatus struct {
	v1.ResourceStatus `json:",inline"`
	AtProvider        ServiceCredentialBindingObservation `json:"atProvider,omitempty"`
}

type SCBResource struct {
	// The GUID of the Cloud Foundry resource
	GUID string `json:"guid,omitempty"`
	// The date and time when the resource was created.
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`
}

// +kubebuilder:object:root=true

// ServiceCredentialBinding is the Schema for the ServiceCredentialBindings API. Provides a Cloud Foundry Service Key.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,cloudfoundry}
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

// Implements Referenceable interface
func (s *ServiceCredentialBinding) GetID() string {
	return s.Status.AtProvider.GUID
}
