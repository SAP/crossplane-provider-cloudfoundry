package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	v2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

type RouteObservation struct {
	Resource `json:",inline"`

	// (String) The protocol of the route.
	// +kubebuilder:validation:Optional
	Protocol *string `json:"protocol,omitempty"`

	// (String) The host name of the route.
	// +kubebuilder:validation:Optional
	Host *string `json:"host,omitempty"`

	// (String) The path of the route.
	// +kubebuilder:validation:Optional
	Path *string `json:"path,omitempty"`

	// (String) The URL of the route.
	// +kubebuilder:validation:Optional
	URL *string `json:"url,omitempty"`

	// (Attributes) The route options.
	// +kubebuilder:validation:Optional
	Options *RouteOptions `json:"options,omitempty"`

	// (List of Attributes) One or more route mappings that map this route to applications. Can be repeated to load balance route traffic among multiple applications.
	// +kubebuilder:validation:Optional
	Destinations []RouteDestination `json:"destinations,omitempty"`
}

type RouteParameters struct {
	SpaceReference `json:",inline"`

	DomainReference `json:",inline"`

	// (String) The application's host name. Required for shared domains.
	// +kubebuilder:validation:Optional
	Host *string `json:"host,omitempty"`

	// (String) A path for an HTTP route.
	// +kubebuilder:validation:Optional
	Path *string `json:"path,omitempty"`

	// (Integer) The port to associate with the route for a TCP route. Conflicts with `random_port`.
	// +kubebuilder:validation:Optional
	Port *int `json:"port,omitempty"`

	// (Attributes) The route options.
	// +kubebuilder:validation:Optional
	Options *RouteOptions `json:"options,omitempty"`
}

type RouteOptions struct {
	// (String) The load balancer associated with this route. Valid values are `round-robin` and `least-connections`.
	// +kubebuilder:validation:Optional
	LoadBalancing string `json:"loadbalancing,omitempty"`
}

type RouteDestination struct {
	// (String) The destination GUID.
	GUID string `json:"guid,omitempty"`

	// (Attributes) The application to map this route to.
	// +kubebuilder:validation:Required
	App *RouteDestinationApp `json:"app,omitempty"`

	// (Integer) The port to associate with the route for a TCP route. Conflicts with `random_port`.
	// +kubebuilder:validation:Optional
	Port *int `json:"port,omitempty"`
}

type RouteDestinationApp struct {
	// (String) The application GUID.
	GUID string `json:"guid,omitempty"`

	// (String) The process type of the destination.
	// +kubebuilder:validation:Optional
	Process *string `json:"process,omitempty"`

	// (Integer) Port on the destination application.
	// +kubebuilder:validation:Optional
	Port *int `json:"port,omitempty"`

	// (String) The protocol for the destination application.
	Protocol *string `json:"protocol,omitempty"`
}

// RouteSpec defines the desired state of Route
type RouteSpec struct {
	v2.ManagedResourceSpec `json:",inline"`
	ForProvider     RouteParameters `json:"forProvider"`
}

// RouteStatus defines the observed state of Route.
type RouteStatus struct {
	v1.ResourceStatus `json:",inline"`
	AtProvider        RouteObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion

// Route is the Schema for the Routes API. Provides a Cloud Foundry route resource.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,cloudfoundry}
// +kubebuilder:validation:XValidation:rule="self.spec.managementPolicies == ['Observe'] || (has(self.spec.forProvider.spaceName) || has(self.spec.forProvider.spaceRef) || has(self.spec.forProvider.spaceSelector))",message="SpaceReference is required: exactly one of spaceName, spaceRef, or spaceSelector must be set"
// +kubebuilder:validation:XValidation:rule="[has(self.spec.forProvider.spaceName), has(self.spec.forProvider.spaceRef), has(self.spec.forProvider.spaceSelector)].filter(x, x).size() <= 1",message="SpaceReference validation: only one of spaceName, spaceRef, or spaceSelector can be set"
// +kubebuilder:validation:XValidation:rule="self.spec.managementPolicies == ['Observe'] || (has(self.spec.forProvider.domainName) || has(self.spec.forProvider.domainRef) || has(self.spec.forProvider.domainSelector))",message="DomainReference is required: exactly one of domainName, domainRef, or domainSelector must be set"
// +kubebuilder:validation:XValidation:rule="[has(self.spec.forProvider.domainName), has(self.spec.forProvider.domainRef), has(self.spec.forProvider.domainSelector)].filter(x, x).size() <= 1",message="DomainReference validation: only one of domainName, domainRef, or domainSelector can be set"
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

// GetID returns the ID of the route
func (r *Route) GetID() string {
	return r.Status.AtProvider.GUID
}

// GetCloudFoundryName implements Namable reference interface
func (r *Route) GetCloudFoundryName() string {
	if r.Status.AtProvider.URL != nil {
		return *r.Status.AtProvider.URL
	}
	return ""
}

// implement DomainScoped interface
func (r *Route) GetDomainRef() *DomainReference {
	return &r.Spec.ForProvider.DomainReference
}
