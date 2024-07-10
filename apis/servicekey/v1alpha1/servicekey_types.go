/*
Copyright 2023 SAP SE.
*/

package v1alpha1

import (
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ServiceKeyObservation defines the observed state of ServiceKey
type ServiceKeyObservation struct {
	// The GUID of the service instance.
	ID *string `json:"id,omitempty" tf:"id,omitempty"`
}

// ServiceKeyParameters define the desired state of the forProvider field of ServiceKey
type ServiceKeyParameters struct {
	// The name of the Service Key in Cloud Foundry.
	// +kubebuilder:validation:Optional
	Name *string `json:"name,omitempty"`

	// Arbitrary parameters in the form of stringified JSON object to pass to the service bind handler.
	// +kubebuilder:validation:Optional
	ParamsJSONSecretRef *v1.SecretKeySelector `json:"paramsJsonSecretRef,omitempty"`

	// A list of key/value parameters used by the service broker to create the binding for the key. By default, no parameters are provided.
	// +kubebuilder:validation:Optional
	ParamsSecretRef *v1.SecretReference `json:"paramsSecretRef,omitempty"`

	// The ID of the Service Instance the key should be associated with.
	// +crossplane:generate:reference:type=github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/service/v1alpha1.ServiceInstance
	// +kubebuilder:validation:Optional
	ServiceInstance *string `json:"serviceInstance,omitempty"`

	// Reference to a ManagedServiceInstance to populate serviceInstance.
	// +kubebuilder:validation:Optional
	ServiceInstanceRef *v1.Reference `json:"serviceInstanceRef,omitempty"`

	// Selector for a ManagedServiceInstance to populate serviceInstance.
	// +kubebuilder:validation:Optional
	ServiceInstanceSelector *v1.Selector `json:"serviceInstanceSelector,omitempty"`

	// Whether or not to output the connectionDetails as a single key with a json object rather than a flat map
	// +kubebuilder:validation:Optional
	ConnectionDetailsAsJSON bool `json:"connectionDetailsAsJSON,omitempty"`
}

// ServiceKeySpec defines the desired state of ServiceKey
type ServiceKeySpec struct {
	v1.ResourceSpec `json:",inline"`
	ForProvider     ServiceKeyParameters `json:"forProvider"`
}

// ServiceKeyStatus defines the observed state of ServiceKey.
type ServiceKeyStatus struct {
	v1.ResourceStatus `json:",inline"`
	AtProvider        ServiceKeyObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// ServiceKey is the Schema for the ServiceKeys API. Provides a Cloud Foundry Service Key.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,cloudfoundry}
type ServiceKey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ServiceKeySpec   `json:"spec"`
	Status            ServiceKeyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ServiceKeyList contains a list of ServiceKeys
type ServiceKeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceKey `json:"items"`
}

// Repository type metadata.
var (
	ServiceKeyKind             = "ServiceKey"
	ServiceKeyGroupKind        = schema.GroupKind{Group: Group, Kind: ServiceKeyKind}.String()
	ServiceKeyKindAPIVersion   = ServiceKeyKind + "." + GroupVersion.String()
	ServiceKeyGroupVersionKind = GroupVersion.WithKind(ServiceKeyKind)
)

func init() {
	SchemeBuilder.Register(&ServiceKey{}, &ServiceKeyList{})
}
