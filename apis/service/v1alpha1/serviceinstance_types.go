/*
Copyright 2023 SAP SE
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// A ServiceInstanceType defines the type of Cloud Foundry service instance type
// +kubebuilder:validation:Enum=managed;user-provided
type ServiceInstanceType string

const (
	// ManagedService manes the external resource is a managed service instance.
	ManagedService ServiceInstanceType = "managed"

	// UserProvidedService means the external resource is a user-provided service instance.
	UserProvidedService ServiceInstanceType = "user-provided"
)

const (
	// LastOperationCreate for create
	LastOperationCreate = "create"

	// LastOperationUpdate for update
	LastOperationUpdate = "update"

	// LastOperationDelete for delete
	LastOperationDelete = "delete"

	// LastOperationInitial signals that the last operation type is initialized
	LastOperationInitial = "initial"

	// LastOperationInProgress signals that the last operation type is in progress
	LastOperationInProgress = "in progress"

	// LastOperationSucceeded signals that the last operation type has succeeded
	LastOperationSucceeded = "succeeded"

	// LastOperationFailed signals that the last operation type has failed
	LastOperationFailed = "failed"
)

// LastOperation records the last performed operation type and state on the service instance.
type LastOperation struct {
	// The last operation performed on the resource
	Type string `json:"type,omitempty"`

	// Last operation state
	State string `json:"state,omitempty"`

	// Description of the last operation
	Description string `json:"description,omitempty"`

	// the time the last operation was performed
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// ServiceInstanceObservation is the type Service Instance status.
type ServiceInstanceObservation struct {

	// The GUID of the service instance
	ID *string `json:"id,omitempty"`

	// The GUID of the Service Plan for a managed service
	ServicePlan *string `json:"servicePlan,omitempty"`

	// The applied parameters/credentials of the service instance
	Credentials []byte `json:"credentials,omitempty"`

	// The job GUID of the last async operation performed on the resource
	LastAsyncJob *string `json:"lastAsyncJob,omitempty"`

	// The last operation performed on the resource
	LastOperation LastOperation `json:"lastOperation"`
}

// ServiceInstanceParameters defines the desired state of Service Instance.
type ServiceInstanceParameters struct {

	// The name of the Service Instance in Cloud Foundry
	// +kubebuilder:validation:Required
	Name *string `json:"name"`

	// The type of the Service Instance in Cloud Foundry. It can be either `managed` or `user-provided`. Default to `managed``
	// +required
	// +kubebuilder:default=managed
	Type ServiceInstanceType `json:"type"`

	// The ID of the space
	// +crossplane:generate:reference:type=github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/space/v1alpha1.Space
	// +crossplane:generate:reference:extractor=github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/config.ExternalID()
	// +kubebuilder:validation:Optional
	Space *string `json:"space,omitempty"`

	// Reference to a Space resource to populate space.
	// +kubebuilder:validation:Optional
	SpaceRef *v1.Reference `json:"spaceRef,omitempty"`

	// Selector for a Space resource to populate space.
	// +kubebuilder:validation:Optional
	SpaceSelector *v1.Selector `json:"spaceSelector,omitempty"`

	// List of instance tags. Some services provide a list of tags that Cloud Foundry delivers in VCAP_SERVICES Env variables. By default, no tags are assigned.
	// +kubebuilder:validation:Optional
	Tags []*string `json:"tags,omitempty" tf:"tags,omitempty"`

	// Fields relevant only for managed service instances
	Managed `json:",inline"`

	// Fields relevant only for user-provided service instances
	UserProvided `json:",inline"`
}

// Managed defines parameters only valid for a managed service instance
type Managed struct {
	// The service plan from which to create the managed service instance. Required for managed service instance type. Service plan can be defined using the GUID, or using a lookup on service offering and service plan names.
	// +optional
	ServicePlan *ServicePlan `json:"servicePlan,omitempty"`

	// Arbitrary service-specific parameters in form of JSON string, passed to the service broker to create the managed service instance.
	// +optional
	JSONParams *string `json:"jsonParams,omitempty"`

	// Arbitrary service-specific parameters in form of secret reference, passed to the service broker to create the managed service instance. Ignored if `jsonParams` is specified
	// +optional
	ParamsSecretRef *SecretReference `json:"paramsSecretRef,omitempty"`
}

// UserProvided defines parameters applicable for user-provided service instance
type UserProvided struct {
	// Arbitrary credentials delivered to applications via VCAP_SERVICES environment variables.Applicable for user-provided service instance type. Credentials can be supplied as JSON object either from string or from SecretRef.
	// +optional
	JSONCredentials *string `json:"jsonCredentials,omitempty"`

	// Arbitrary service-specific parameters in form of secret reference, passed to the service broker to create the managed service instance. Ignored if `jsonParams` is specified
	// +optional
	CredentialsSecretRef *SecretReference `json:"credentialsSecretRef,omitempty"`

	// URL to which requests for bound routes will be forwarded. Scheme for this URL must be https and defaults to empty
	// +optional
	RouteServiceURL string `json:"routeServiceUrl,omitempty"`

	// URL to which logs for bound applications will be streamed. Defaults to empty.
	// +optional
	SyslogDrainURL string `json:"syslogDrainUrl,omitempty"`
}

// SecretReference defines parameters applicable for user-provided service instance
type SecretReference struct {
	// Name of the secret.
	Name string `json:"name"`

	// Namespace of the secret.
	Namespace string `json:"namespace"`

	// The optional key to select. If key is not specified, the entire key-value map of secret is used
	// +optional
	Key *string `json:"key,omitempty"`
}

// ServicePlan define a service plan
type ServicePlan struct {
	// The ID of the service plan
	// +optional
	ID *string `json:"id"`

	// The name of service offering
	// +optional
	Offering *string `json:"offering"`

	// The name of service plan
	// +optional
	Plan *string `json:"plan"`
}

// ServiceInstanceSpec defines the desired state of ServiceInstance
type ServiceInstanceSpec struct {
	v1.ResourceSpec `json:",inline"`
	ForProvider     ServiceInstanceParameters `json:"forProvider"`
}

// ServiceInstanceStatus defines the observed state of ServiceInstance.
type ServiceInstanceStatus struct {
	v1.ResourceStatus `json:",inline"`
	AtProvider        ServiceInstanceObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// ServiceInstance is the Schema for the ServiceInstances API. Provides a Cloud Foundry Service Instance.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,cloudfoundry}
type ServiceInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ServiceInstanceSpec   `json:"spec"`
	Status            ServiceInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ServiceInstanceList contains a list of ServiceInstances
type ServiceInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceInstance `json:"items"`
}

// Repository type metadata.
var (
	ServiceInstanceKind             = "ServiceInstance"
	ServiceInstanceGroupKind        = schema.GroupKind{Group: Group, Kind: ServiceInstanceKind}.String()
	ServiceInstanceKindAPIVersion   = ServiceInstanceKind + "." + GroupVersion.String()
	ServiceInstanceGroupVersionKind = GroupVersion.WithKind(ServiceInstanceKind)
)

func init() {
	SchemeBuilder.Register(&ServiceInstance{}, &ServiceInstanceList{})
}
