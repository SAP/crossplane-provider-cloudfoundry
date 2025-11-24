package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
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
	RouteReference `json:",inline,omitempty"`

	ServiceInstanceReference `json:",inline,omitempty"`

	ResourceMetadata `json:",inline"`

	// A map of arbitrary key/value paris to be send to the service broker during binding
	// +kubebuilder:validation:Optional
	Parameters runtime.RawExtension `json:"parameters,omitempty"`
}

type ServiceRouteBindingObservation struct {
	Resource `json:",inline"`

	RouteServiceUrl string `json:"routeServiceUrl"`

	LastOperation *LastOperation `json:"lastOperation,omitempty"`

	ResourceMetadata `json:",inline"`

	Links Links `json:"links,omitempty"`

	// GUID of the ServiceRouteBinding in CF
	// +kubebuilder:validation:Optional
	ServiceInstance string `json:"serviceInstanceGUID,omitempty"`

	// GUID of the Route in CF
	// +kubebuilder:validation:Optional
	Route string `json:"routeGUID,omitempty"`

	// TODO: Parameters are not returned from CF API so most likely we cannot store them here???
	// Solutin: GET /v3/service_route_bindings/:guid/parameters
	// A map of arbitrary key/value paris to be send to the service broker during binding
	// +kubebuilder:validation:Optional
	Parameters runtime.RawExtension `json:"parameters,omitempty"`
}

type Relation struct {
	ServiceInstance Data `json:"service_instance"`
	Route           Data `json:"route"`
}

type Data struct {
	GUID string `json:"guid"`
}

type Link struct {
	Href string `json:"href"`

	// +kubebuilder:validation:Optional
	Method *string `json:"method,omitempty"`
}

type Links map[string]Link

// ServiceRouteBindingSpec defines the desired state of ServiceRouteBinding
type ServiceRouteBindingSpec struct {
	xpv1.ResourceSpec `json:",inline"`
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
	//GUID of the ServiceInstance in CF
	// +crossplane:generate:reference:type=ServiceInstance
	// +crossplane:generate:reference:extractor=github.com/SAP/crossplane-provider-cloudfoundry/apis/resources.ExternalID()
	ServiceInstance         string          `json:"serviceInstance,omitempty"`
	ServiceInstanceRef      *xpv1.Reference `json:"serviceInstanceRef,omitempty"`
	ServiceInstanceSelector *xpv1.Selector  `json:"serviceInstanceSelector,omitempty"`
}

type RouteReference struct {
	//GUID of the Route in CF
	// +crossplane:generate:reference:type=Route
	// +crossplane:generate:reference:extractor=github.com/SAP/crossplane-provider-cloudfoundry/apis/resources.ExternalID()
	Route         string          `json:"route,omitempty"`
	RouteRef      *xpv1.Reference `json:"routeRef,omitempty"`
	RouteSelector *xpv1.Selector  `json:"routeSelector,omitempty"`
}
