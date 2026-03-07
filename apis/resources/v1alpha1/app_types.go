/*
Copyright 2023 SAP SE.
*/
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	v2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

type AppObservation struct {
	Resource `json:",inline"`

	// The `name` of the application.
	Name string `json:"name,omitempty"`

	// the `state` of the application.
	State string `json:"state,omitempty"`

	// The yaml representation of the environment variables.
	AppManifest string `json:"appManifest,omitempty"`
}

type AppParameters struct {
	// The `name` of the application.
	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`

	// Type of the lifecycle; valid values are `buildpack`, `cnb`, `docker`
	// +kubebuilder:validation:Enum=buildpack;cnb;docker
	// +kubebuilder:default=buildpack
	Lifecycle string `json:"lifecycle,omitempty"`

	SpaceReference `json:",inline"`

	// (NOT SUPPORTED YET) An array of one ore more installed buildpack names, e.g., ruby_buildpack, java_buildpack.
	// +kubebuilder:validation:Optional
	Buildpacks []string `json:"buildpacks,omitempty"`

	// (NOT SUPPORTED YET) The root filesystem to use with the buildpack, for example, cflinuxfs4.
	// +kubebuilder:validation:Optional
	Stack *string `json:"stack,omitempty"`

	// (NOT SUPPORTED YET) The path to the app directory or zip file to push.
	// +kubebuilder:validation:Optional
	Path *string `json:"path,omitempty"`

	// Specifies docker image and optional docker credentials when lifecycle is set to docker
	// +kubebuilder:validation:Optional
	Docker *DockerConfiguration `json:"docker,omitempty"`

	// When set to true, any routes configuration specified in the manifest will be ignored and any existing routes will be removed.
	// +kubebuilder:validation:Optional
	NoRoute bool `json:"no-route,omitempty"`

	// (NOT SUPPORTED YET) The routes to map to the application to control its ingress traffic.
	// +kubebuilder:validation:Optional
	Routes []RouteConfiguration `json:"routes,omitempty"`

	// When set to true, a random route will be created and mapped to the application. Ignored if routes are specified, or if no-route is set to true.
	// +kubebuilder:validation:Optional
	RandomRoute bool `json:"random-route,omitempty"`

	// When set to true, a route for the app will be created using the app name as the hostname and the containing org's default domain as the domain. Ignored if routes are specified or if no-route is set to true.
	// +kubebuilder:validation:Optional
	DefaultRoute bool `json:"default-route,omitempty"`

	// (NOT SUPPORTED YET) Service instances to bind to the application.
	// +kubebuilder:validation:Optional
	Services []ServiceBindingConfiguration `json:"services,omitempty"`

	// Configure single process for the application.
	// +kubebuilder:validation:Optional
	//	ProcessConfiguration `json:",inline"`

	// Configures multiple processes to run for an App. For example, a web application may have a web UI process and a worker process.
	// +kubebuilder:validation:Optional
	Processes []ProcessConfiguration `json:"processes,omitempty"`

	// Readiness health check configuration for the application.
	// +kubebuilder:validation:Optional
	ReadinessHealthCheckConfiguration `json:",inline"`

	// Sidecar configuration for the application.
	// +kubebuilder:validation:Optional
	// Sidecars []SidecarConfiguration `json:"sidecars,omitempty"`

	// (NOT SUPPORTED YET) A key-value mapping of environment variables to be used for the app when running
	// +kubebuilder:validation:Optional
	Environment *runtime.RawExtension `json:"environment,omitempty"`

	// The log rate limit for all instances of an app. This attribute requires a unit of measurement: B, K, KB, M, MB, G, or GB, in either uppercase or lowercase.
	// +kubebuilder:validation:Optional
	LogRateLimitPerSecond *string `json:"log-rate-limit-per-second,omitempty"`

	ResourceMetadata `json:",inline"`
}

type DockerConfiguration struct {
	// The URL to the docker image with tag e.g registry.example.com:5000/user/repository/tag or docker image name from the public repo e.g. redis:4.0
	// +kubebuilder:validation:Required
	Image string `json:"image,omitempty"`

	// (Attributes) Defines login credentials for private docker repositories
	// +kubebuilder:validation:Optional
	Credentials *v1.SecretReference `json:"credentialsSecretRef,omitempty"`
}

// RouteConfiguration defines the route for the application
type RouteConfiguration struct {
	// (Number) The port of the application to map the tcp route to.
	// +kubebuilder:validation:Optional
	Protocol *string `json:"protocol,omitempty"`

	// The route id. Route can be defined using the cloudfoundry_route resource
	// +crossplane:generate:reference:type=Route
	// +crossplane:generate:reference:extractor=github.com/SAP/crossplane-provider-cloudfoundry/apis/resources.CloudFoundryName()
	// +kubebuilder:validation:Optional
	Route *string `json:"route,omitempty"`

	// Reference to a Route in route to populate route.
	// +kubebuilder:validation:Optional
	RouteRef *v1.NamespacedReference `json:"routeRef,omitempty"`

	// Selector for a Route in route to populate route.
	// +kubebuilder:validation:Optional
	RouteSelector *v1.NamespacedSelector `json:"routeSelector,omitempty"`
}

// ServiceBindingConfiguration defines the service instance to bind to the application
type ServiceBindingConfiguration struct {
	// The name of the service instance to be bound to.
	// +crossplane:generate:reference:type=ServiceInstance
	// +crossplane:generate:reference:refFieldName=ServiceInstanceRef
	// +crossplane:generate:reference:selectorFieldName=ServiceInstanceSelector
	// +crossplane:generate:reference:extractor=github.com/SAP/crossplane-provider-cloudfoundry/apis/resources.CloudFoundryName()
	Name *string `json:"name,omitempty"`

	// Reference to a ServiceInstance in service to populate serviceInstance.
	// +kubebuilder:validation:Optional
	ServiceInstanceRef *v1.NamespacedReference `json:"serviceInstanceRef,omitempty"`

	// Selector for a ServiceInstance in service to populate serviceInstance.
	// +kubebuilder:validation:Optional
	ServiceInstanceSelector *v1.NamespacedSelector `json:"serviceInstanceSelector,omitempty"`

	// The name of the service instance to bind to the application.
	// +kubebuilder:validation:Optional
	BindingName string `json:"binding_name,omitempty"`

	// A map of arbitrary key/value paris to be send to the service broker during binding
	// +kubebuilder:validation:Optional
	Parameters runtime.RawExtension `json:"parameters,omitempty"`
}

// HealthCheckConfiguration defines the health check configuration for the application
type HealthCheckConfiguration struct {
	// The type of health check to perform, either http or tcp or process.
	// +kubebuilder:validation:Enum=http;port;process
	// +kubebuilder:default:port
	HealthCheckType *string `json:"health-check-type,omitempty"`

	// The endpoint called to determine if the app is healthy
	// +kubebuilder:validation:Optional
	HealthCheckHTTPEndpoint *string `json:"health-check-http-endpoint,omitempty"`

	// The interval in seconds between health checks
	// +kubebuilder:validation:Optional
	HealthCheckInterval *uint `json:"health-check-interval,omitempty"`

	// Timeout in seconds for individual health check requests
	// +kubebuilder:validation:Optional
	HealthCheckInvocationTimeout *uint `json:"health-check-invocation-timeout,omitempty"`
}

// ReadinessHealthCheckConfiguration defines the health check configuration for the application
type ReadinessHealthCheckConfiguration struct {
	// The type of readiness health check to perform, either http or tcp or process.
	// +kubebuilder:validation:Enum=http;port;process
	// +kubebuilder:default:port
	ReadinessHealthCheckType *string `json:"readiness-health-check-type,omitempty"`

	// The endpoint called to determine if the app is ready
	// +kubebuilder:validation:Optional
	ReadinessHealthCheckHTTPEndpoint *string `json:"readiness-health-check-http-endpoint,omitempty"`

	// The interval in seconds between readiness health checks
	// +kubebuilder:validation:Optional
	ReadinessHealthCheckInterval *uint `json:"readiness-health-check-interval,omitempty"`

	// Timeout in seconds for individual readiness health check requests
	// +kubebuilder:validation:Optional
	ReadinessHealthCheckInvocationTimeout *uint `json:"readiness-health-check-invocation-timeout,omitempty"`
}

// ProcessConfiguration defines the process-level configuration  for the application
type ProcessConfiguration struct {
	// The identifier for the process to be configured.
	// +kubebuilder:validation:optional
	Type *string `json:"type"`

	// The command used to start the process.
	// +kubebuilder:validation:Optional
	Command *string `json:"command,omitempty"`

	// The disk limit for all instance of the web process type. This attribute requires a unit of measurement, such as M, MB, G, GB, T, or TB in upper case or lower case.
	// +kubebuilder:validation:Optional
	DiskQuota *string `json:"diskQuota,omitempty"`

	// The number of instances of the process to run.
	// +kubebuilder:validation:Optional
	Instances *uint `json:"instances,omitempty"`

	// The amount of memory allocated to each instance of the process. This attribute requires a unit of measurement, such as M, MB, G, GB, T, or TB in upper case or lower case.
	// +kubebuilder:validation:Optional
	Memory *string `json:"memory,omitempty"`

	// Timeout in seconds at which the health check is considered a failure
	// +kubebuilder:validation:Optional
	Timeout *uint `json:"timeout,omitempty"`

	HealthCheckConfiguration `json:",inline"`
}

// SidecarConfiguration defines the sidecar configuration for the application
type SidecarConfiguration struct {
	// The name of the sidecar process to be configured.
	// +kubebuilder:validation:required
	Name string `json:"name"`

	// The command used to start the sidecar process.
	// +kubebuilder:validation:Optional
	Command *string `json:"command,omitempty"`

	// List of processes to associate with the sidecar.
	ProcessTypes []string `json:"process-types"`

	// Memory in MB to be allocated to the sidecar.
	Memory *uint `json:"memory"`
}

// AppSpec defines the desired state of App
type AppSpec struct {
	v2.ManagedResourceSpec `json:",inline"`
	ForProvider     AppParameters `json:"forProvider"`
}

// AppStatus defines the observed state of App.
type AppStatus struct {
	v1.ResourceStatus `json:",inline"`
	AtProvider        AppObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// App is the Schema for the Apps API. Provides a Cloud Foundry resource to manage applications.
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,cloudfoundry}
// +kubebuilder:validation:XValidation:rule="self.spec.managementPolicies == ['Observe'] || (has(self.spec.forProvider.spaceName) || has(self.spec.forProvider.spaceRef) || has(self.spec.forProvider.spaceSelector))",message="SpaceReference is required: exactly one of spaceName, spaceRef, or spaceSelector must be set"
// +kubebuilder:validation:XValidation:rule="[has(self.spec.forProvider.spaceName), has(self.spec.forProvider.spaceRef), has(self.spec.forProvider.spaceSelector)].filter(x, x).size() <= 1",message="SpaceReference validation: only one of spaceName, spaceRef, or spaceSelector can be set"
type App struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              AppSpec   `json:"spec"`
	Status            AppStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AppList contains a list of Apps
type AppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []App `json:"items"`
}

// Repository type metadata.
var (
	App_Kind             = "App"
	App_GroupKind        = schema.GroupKind{Group: CRDGroup, Kind: App_Kind}.String()
	App_KindAPIVersion   = App_Kind + "." + CRDGroupVersion.String()
	App_GroupVersionKind = CRDGroupVersion.WithKind(App_Kind)
)

func init() {
	SchemeBuilder.Register(&App{}, &AppList{})
}

// implement Referenceable interface
func (s *App) GetID() string {
	return s.Status.AtProvider.GUID
}

// implement SpaceScoped interface
func (s *App) GetSpaceRef() *SpaceReference {
	return &s.Spec.ForProvider.SpaceReference
}
