package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	//TODO: we most likely want to reference a other resource here and not just have a string
	RouteServiceUrl string `json:"route_service_url"`

	ResourceMetadata `json:",inline"`

	Relationships Relation `json:"relationships"`

	Links Links `json:"links,omitempty"`
}

type ServiceRouteBindingObservation struct {
	Resource `json:",inline"`

	//TODO: we most likely want to reference a other resource here and not just have a string
	RouteServiceUrl string `json:"route_service_url"`

	LastOperation *LastOperation `json:"lastOperation,omitempty"`

	ResourceMetadata `json:",inline"`

	Relationships Relation `json:"relationships"`

	Links Links `json:"links,omitempty"`
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

// -------------------------------------------------------------------------------------------------
// Link modeling options:
// We need a required 'self' link plus any number of additional dynamic links returned by CF (e.g. service_instance, route, parameters).
// Option 1 uses a flat map (LinksMap) matching CF JSON exactly, but cannot enforce 'self' at schema level (must validate in controller).
// Option 2 (active) uses a struct with a required Self field and an 'additional' map to hold any other links, enabling schema enforcement.
// TODO: find out if its just service_instance, route, parameters fields (Typed) or dynamic keys!!!
// check out proposed solution https://github.com/SAP/crossplane-provider-cloudfoundry/issues/81

// Option 1:
type LinksMap map[string]Link

// Option 2: Struct-based enforced 'self' (CURRENTLY ACTIVE)
type Links struct {
	// +kubebuilder:validation:Required
	Self Link `json:"self"`
	// Additional dynamic links (e.g. service_instance, route, parameters, future).
	// +kubebuilder:validation:Optional
	Additional map[string]Link `json:"additional,omitempty"`
}

// -------------------------------------------------------------------------------------------------

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
