/*
Copyright 2023 SAP SE.
*/

package v1alpha1

import (
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// RotatingCredentialBindingParameters define the desired state of the forProvider field of RotatingCredentialBinding
type RotatingCredentialBindingParameters struct {
	// The name of the Service Key in Cloud Foundry.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// The namespace of the Service Key in Cloud Foundry.
	// +kubebuilder:validation:Optional
	Namespace *string `json:"namespace,omitempty"`

	// The ID of the Service Instance the key should be associated with.
	// +crossplane:generate:reference:type=github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1.ServiceInstance
	// +kubebuilder:validation:Optional
	ServiceInstance *string `json:"serviceInstance,omitempty"`

	// Reference to a ManagedServiceInstance to populate serviceInstance.
	// +kubebuilder:validation:Optional
	ServiceInstanceRef *v1.Reference `json:"serviceInstanceRef,omitempty"`

	// Selector for a ManagedServiceInstance to populate serviceInstance.
	// +kubebuilder:validation:Optional
	ServiceInstanceSelector *v1.Selector `json:"serviceInstanceSelector,omitempty"`

	// An optional JSON object to pass parameters to the service broker .
	// +kubebuilder:validation:Optional
	Parameters *runtime.RawExtension `json:"parameters,omitempty"`

	// Use a reference to a secret to pass parameters to the service broker. Ignored if parameters is set.
	// +kubebuilder:validation:Optional
	ParametersSecretRef *v1.SecretReference `json:"paramsSecretRef,omitempty"`
}

// RotatingCredentialBindingSpec defines the desired state of RotatingCredentialBinding
type RotatingCredentialBindingSpec struct {
	v1.ResourceSpec `json:",inline"`

	// True to write connectionDetails as single key-value in a secret rather than a map. The key is the metadata.name of the service credential binding CR itself.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	ConnectionDetailsAsJSON bool `json:"connectionDetailsAsJSON,omitempty"`

	ForProvider RotatingCredentialBindingParameters `json:"forProvider"`

	// RotationFrequency is the frequency at which the credentials should be rotated.
	// +kubebuilder:validation:Required
	RotationFrequency *metav1.Duration `json:"rotationFrequency"`

	// RotationTTL is the time to live for the credentials after rotation.
	// This is the time after rotation which the old credentials will be deleted.
	// +kubebuilder:validation:Required
	RotationTTL *metav1.Duration `json:"rotationTTL"`
}

// RotatingCredentialBindingStatus defines the observed state of RotatingCredentialBinding.
type RotatingCredentialBindingStatus struct {
	v1.ResourceStatus `json:",inline"`

	// Active Service Credential Binding Reference.
	// +kubebuilder:validation:Optional
	ActiveServiceCredentialBinding *ServiceCredentialBindingReference `json:"activeServiceCredentialBinding,omitempty"`

	// Prevously rotated Service Credential Bindings.
	// +kubebuilder:validation:Optional
	PreviousServiceCredentialBindings []*ServiceCredentialBindingReference `json:"previousServiceCredentialBindings,omitempty"`
}

type ServiceCredentialBindingReference struct {
	// Name of the Service Credential Binding
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the Service Credential Binding
	// +kubebuilder:validation:Optional
	Namespace string `json:"namespace,omitempty"`

	// Last rotation time of the Service Credential Binding
	LastRotation metav1.Time `json:"lastRotation,omitempty"`
}

// +kubebuilder:object:root=true

// RotatingCredentialBinding manages the lifecycle of a Cloud Foundry Service Credential Binding that supports blue-green secret rotation.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,cloudfoundry}
type RotatingCredentialBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RotatingCredentialBindingSpec   `json:"spec"`
	Status            RotatingCredentialBindingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RotatingCredentialBindingList contains a list of RotatingCredentialBindings
type RotatingCredentialBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RotatingCredentialBinding `json:"items"`
}

// Repository type metadata.
var (
	RotatingCredentialBindingKind             = "RotatingCredentialBinding"
	RotatingCredentialBindingGroupKind        = schema.GroupKind{Group: CRDGroup, Kind: RotatingCredentialBindingKind}.String()
	RotatingCredentialBindingKindAPIVersion   = RotatingCredentialBindingKind + "." + CRDGroupVersion.String()
	RotatingCredentialBindingGroupVersionKind = CRDGroupVersion.WithKind(RotatingCredentialBindingKind)
)

func init() {
	SchemeBuilder.Register(&RotatingCredentialBinding{}, &RotatingCredentialBindingList{})
}

// Implements Referenceable interface
// func (s *RotatingCredentialBinding) GetID() string {
// 	return s.Status.AtProvider.GUID
// }
