package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// RouteObservation observations for routes
type RouteObservation struct {

	// The complete endpoint with path if set for the route
	Endpoint *string `json:"endpoint,omitempty"`

	// The GUID of the route
	ID *string `json:"id,omitempty"`
}

// RouteParameters parameters for Routes
type RouteParameters struct {

	// The domain to map the host name to. If not provided the default application domain will be used.
	// +kubebuilder:validation:Required
	Domain DomainParameters `json:"domain,omitempty"`

	// The application's host name. This is required for shared domains.
	// +kubebuilder:validation:Optional
	Hostname *string `json:"hostname,omitempty"`

	// A path for an HTTP route.
	// +kubebuilder:validation:Optional
	Path *string `json:"path,omitempty"`

	// The port to associate with the route for a TCP route. Conflicts with random_port.
	// +kubebuilder:validation:Optional
	Port *int `json:"port,omitempty"`

	// The ID of the space to create the route in.
	// +crossplane:generate:reference:type=github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/resources/v1alpha2.Space
	// +crossplane:generate:reference:extractor=github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/config.ExternalID()
	// +kubebuilder:validation:Optional
	Space *string `json:"space,omitempty"`

	// Reference to a Space in space to populate space.
	// +kubebuilder:validation:Optional
	SpaceRef *v1.Reference `json:"spaceRef,omitempty"`

	// Selector for a Space in space to populate space.
	// +kubebuilder:validation:Optional
	SpaceSelector *v1.Selector `json:"spaceSelector,omitempty"`

	// One or more route mapping(s) that will map this route to application(s). Can be repeated multiple times to load balance route traffic among multiple applications.
	// +kubebuilder:validation:Optional
	Destinations []DestinationParameters `json:"destinations,omitempty"`
}

// DomainParameters parameters for domain.
type DomainParameters struct {

	// The ID of the Domain.
	// +kubebuilder:validation:Optional
	ID *string `json:"id,omitempty"`

	// The name of the Domain.
	// +kubebuilder:validation:Optional
	Name *string `json:"name,omitempty"`
}

// DestinationObservation observation for destinations
type DestinationObservation struct {
}

// DestinationParameters parameters for Destinations
type DestinationParameters struct {

	// The ID of the application to map this route to.
	// +kubebuilder:validation:Required
	App *string `json:"app"`

	// The port to associate with the route for a TCP route. Conflicts with random_port.
	// +kubebuilder:validation:Optional
	Port *int `json:"port,omitempty"`
}

// RouteSpec defines the desired state of Route
type RouteSpec struct {
	v1.ResourceSpec `json:",inline"`
	ForProvider     RouteParameters `json:"forProvider"`
}

// RouteStatus defines the observed state of Route.
type RouteStatus struct {
	v1.ResourceStatus `json:",inline"`
	AtProvider        RouteObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// Route is the Schema for the Routes API. Provides a Cloud Foundry route resource.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,cloudfoundry}
type Route struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RouteSpec   `json:"spec"`
	Status            RouteStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RouteList contains a list of Routes
type RouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Route `json:"items"`
}

// Repository type metadata.
var (
	RouteKind             = "Route"
	RouteGroupKind        = schema.GroupKind{Group: CRDGroup, Kind: RouteKind}.String()
	RouteKindAPIVersion   = RouteKind + "." + CRDGroupVersion.String()
	RouteGroupVersionKind = CRDGroupVersion.WithKind(RouteKind)
)

func init() {
	SchemeBuilder.Register(&Route{}, &RouteList{})
}
