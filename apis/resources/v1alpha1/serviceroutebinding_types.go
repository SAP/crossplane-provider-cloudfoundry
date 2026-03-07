package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,cloudfoundry}
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="ROUTE",type="string",JSONPath=".status.atProvider.routeGUID",priority=1
// +kubebuilder:printcolumn:name="SERVICE-INSTANCE",type="string",JSONPath=".status.atProvider.serviceInstanceGUID",priority=1
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name",priority=1
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:validation:XValidation:rule="has(self.spec.forProvider.serviceInstance) || has(self.spec.forProvider.serviceInstanceRef) || has(self.spec.forProvider.serviceInstanceSelector)",message="ServiceInstanceReference validation: one of serviceInstance, serviceInstanceRef, or serviceInstanceSelector must be set"
// +kubebuilder:validation:XValidation:rule="!has(self.spec.forProvider.serviceInstanceRef) || !has(self.spec.forProvider.serviceInstanceSelector)",message="ServiceInstanceReference validation: serviceInstanceRef and serviceInstanceSelector are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="has(self.spec.forProvider.route) || has(self.spec.forProvider.routeRef) || has(self.spec.forProvider.routeSelector)",message="RouteReference validation: one of route, routeRef, or routeSelector must be set"
// +kubebuilder:validation:XValidation:rule="!has(self.spec.forProvider.routeRef) || !has(self.spec.forProvider.routeSelector)",message="RouteReference validation: routeRef and routeSelector are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.spec.forProvider.route) || !has(self.spec.forProvider.route) || oldSelf.spec.forProvider.route.size() == 0 || oldSelf.spec.forProvider.route == self.spec.forProvider.route",message="ServiceRouteBinding is immutable: route GUID cannot be changed once set"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.spec.forProvider.serviceInstance) || !has(self.spec.forProvider.serviceInstance) || oldSelf.spec.forProvider.serviceInstance.size() == 0 || oldSelf.spec.forProvider.serviceInstance == self.spec.forProvider.serviceInstance",message="ServiceRouteBinding is immutable: serviceInstance GUID cannot be changed once set"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.spec.forProvider.parameters) || !has(self.spec.forProvider.parameters) || oldSelf.spec.forProvider.parameters == self.spec.forProvider.parameters",message="ServiceRouteBinding is immutable: parameters cannot be changed once set"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.spec.forProvider.paramsSecretRef) || !has(self.spec.forProvider.paramsSecretRef) || oldSelf.spec.forProvider.paramsSecretRef == self.spec.forProvider.paramsSecretRef",message="ServiceRouteBinding is immutable: paramsSecretRef cannot be changed once set"
type ServiceRouteBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ServiceRouteBindingSpec   `json:"spec"`
	Status            ServiceRouteBindingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// ServiceRouteBindingList contains a list of ServiceRouteBindings
type ServiceRouteBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceRouteBinding `json:"items"`
}

type ServiceRouteBindingParameters struct {
	RouteReference `json:",inline"`

	ServiceInstanceReference `json:",inline"`

	ResourceMetadata `json:",inline"`

	// A map of arbitrary key/value paris to be send to the service broker during binding only supported for user-provided service instances
	// +kubebuilder:validation:Optional
	Parameters runtime.RawExtension `json:"parameters,omitempty"`

	// (Attributes) Use a reference to a secret to pass `parameters` to the service broker. Ignored if `parameters` is set.
	// +kubebuilder:validation:Optional
	ParametersSecretRef *xpv1.SecretReference `json:"paramsSecretRef,omitempty"`
}

type ServiceRouteBindingObservation struct {
	Resource `json:",inline"`

	// (String) The URL of the route service if one is associated with the service route binding.
	RouteServiceUrl string `json:"routeServiceUrl"`

	LastOperation *LastOperation `json:"lastOperation,omitempty"`

	ResourceMetadata `json:",inline"`

	// The links related to the ServiceRouteBinding, ServiceInstance, Parameter and Route
	Links Links `json:"links,omitempty"`

	// GUID of the ServiceRouteBinding in CF
	// +kubebuilder:validation:Optional
	ServiceInstance string `json:"serviceInstanceGUID,omitempty"`

	// GUID of the Route in CF
	// +kubebuilder:validation:Optional
	Route string `json:"routeGUID,omitempty"`

	// A map of arbitrary key/value paris to be send to the service broker during binding only supported for user-provided service instances
	// +kubebuilder:validation:Optional
	Parameters runtime.RawExtension `json:"parameters,omitempty"`
}

type Relation struct {
	// wrapper for GUID of ServiceInstance in CF
	ServiceInstance Data `json:"service_instance"`

	// wrapper for GUID of Route in CF
	Route Data `json:"route"`
}

type Data struct {
	// GUID of the related resource in CF
	GUID string `json:"guid"`
}

type Link struct {
	// Contains the href to a related resource in CF
	Href string `json:"href"`

	// (Optional) HTTP method to be used when accessing the link
	// +kubebuilder:validation:Optional
	Method *string `json:"method,omitempty"`
}

// Contains the links related to the ServiceRouteBinding, ServiceInstance, Parameter and Route
type Links map[string]Link

// ServiceRouteBindingSpec defines the desired state of ServiceRouteBinding
type ServiceRouteBindingSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider       ServiceRouteBindingParameters `json:"forProvider"`
}

// ServiceRouteBindingStatus defines the observed state of ServiceRouteBinding
type ServiceRouteBindingStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          ServiceRouteBindingObservation `json:"atProvider,omitempty"`
}

// Repository type metadata for registration.
var (
	ServiceRouteBinding_Kind             = "ServiceRouteBinding"
	ServiceRouteBinding_GroupKind        = schema.GroupKind{Group: CRDGroup, Kind: ServiceRouteBinding_Kind}.String()
	ServiceRouteBinding_KindAPIVersion   = ServiceRouteBinding_Kind + "." + CRDGroupVersion.String()
	ServiceRouteBinding_GroupVersionKind = CRDGroupVersion.WithKind(ServiceRouteBinding_Kind)
)

func init() {
	SchemeBuilder.Register(&ServiceRouteBinding{}, &ServiceRouteBindingList{})
}

type ServiceInstanceReference struct {
	// GUID of the ServiceInstance in CF if ServiceInstanceRef or ServiceInstanceSelector is set it will be overwritten
	// +crossplane:generate:reference:type=ServiceInstance
	// +crossplane:generate:reference:extractor=github.com/SAP/crossplane-provider-cloudfoundry/apis/resources.ExternalID()
	ServiceInstance string `json:"serviceInstance,omitempty"`
	// If set will overwrite ServiceInstance
	ServiceInstanceRef *xpv1.NamespacedReference `json:"serviceInstanceRef,omitempty"`
	// If set will overwrite ServiceInstance
	ServiceInstanceSelector *xpv1.NamespacedSelector `json:"serviceInstanceSelector,omitempty"`
}

type RouteReference struct {
	// GUID of the Route in CF if RouteRef or RouteSelector is set it will be overwritten
	// +crossplane:generate:reference:type=Route
	// +crossplane:generate:reference:extractor=github.com/SAP/crossplane-provider-cloudfoundry/apis/resources.ExternalID()
	Route string `json:"route,omitempty"`
	// If set will overwrite Route
	RouteRef *xpv1.NamespacedReference `json:"routeRef,omitempty"`
	// If set will overwrite Route
	RouteSelector *xpv1.NamespacedSelector `json:"routeSelector,omitempty"`
}
