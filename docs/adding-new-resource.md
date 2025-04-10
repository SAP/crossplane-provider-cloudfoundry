# Adding a New Resource

## Add a new API definition in `apis/resources`

The `apis/resources` directory contains the API definitions of all custom resource for this provider. The api definitions are written in Go and are used to generate the CRDs (Custom Resource Definitions) for the provider. So to add a new resource, the first thing is to add a new type in the `apis/resources` directory. The single source of truth for API definitions is the official [Cloud Foundry API Reference](https://v3-apidocs.cloudfoundry.org/version/3.185.0/#resources).

All Crossplane types have exactly the same root structure. The root type is resource itself and it contains `Spec` and `Status` fields. The `Spec` field contains the `ForProvider` field that describes the desired state of the resource and the `Status` field contains the `AtProvider` that records the current state of the resource.  For example, if we add a new custom resource `Route`, the root type will look like this:

```go
// +kubebuilder:object:root=true
type Route struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RouteSpec   `json:"spec"`
	Status            RouteStatus `json:"status,omitempty"`
}

// RouteSpec defines the desired state of Route
type XxxSpec struct {
    xpv1.ResourceSpec `json:",inline"`
    ForProvider       RouteParameters `json:"forProvider"`
}

// RouteStatus defines the observed state of Route
type RouteStatus struct {
    xpv1.ResourceStatus `json:",inline"`
    AtProvider          RouteObservation `json:"atProvider,omitempty"`
}
```
In most cases, `ForProvider` and `AtProvider` are the only variable definitions you need to provide when defining a new type for your new resources, based on the API reference.

### `ForProvider` must match `API Create` payload

`ForProvider` is defined in the type `xxxParameters`. The fields in this type should match the fields that are used to **create** the resource in Cloud Foundry. For example, `RouteParameters` should include all parameters that one can pass to [create a route](https://v3-apidocs.cloudfoundry.org/version/3.185.0/#create-a-route), including both required parameters and optional parameters.

- Match basic type straight forwardly. For example, if the API requires a string, you should define the field as `string` or `*string` in the `xxxParameters` type.
- Match complex types with other types. For example, if the API requires a `route option` object, you should define a new type `RouteOption` and use it in the `RouteParameters` type.
- Match `to-one relationship` and `to-many relationship` using Crossplane's [cross resource referencing](https://github.com/crossplane/crossplane/blob/main/design/one-pager-cross-resource-referencing.md). For example, creating a *route* requires you to specify relationships to the *space* in which the *route* to be created and to the *domain* of the route.
  
A cross resource referencing to `Space` is defined as following:
```go
	// The ID of the space to create the route in.
	// +crossplane:generate:reference:type=Space
	// +crossplane:generate:reference:extractor=github.com/SAP/crossplane-provider-cloudfoundry/apis/resources.ExternalID()
	// +kubebuilder:validation:Optional
	Space *string `json:"space,omitempty"`

	// Reference to a Space in space to populate space.
	// +kubebuilder:validation:Optional
	SpaceRef *v1.Reference `json:"spaceRef,omitempty"`

	// Selector for a Space in space to populate space.
	// +kubebuilder:validation:Optional
	SpaceSelector *v1.Selector `json:"spaceSelector,omitempty"`
```
Crossplane generates reference resolvers, which will be used by the Crossplane Reconciler to resolve the `Space` reference to the actual `Space` object. The `Space` reference can be specified in the `RouteParameters` type as `SpaceRef` or `SpaceSelector`. Essentially, the resolver simply locates the referenced custom resource and uses provided extract function to resolve `Space` reference. You can examine the generated code in the `zz_generated.resolvers.go` file.

Similarly you can define a reference to `Domain` in the `RouteParameters` type. Some commonly used reference types are to `Space` and to `Org`. These are defined in the `references.go` for reuse.

###  `AtProvider` should orient on `API Update` payload

`AtProvider` is defined in the type `xxxObservation`. The fields in this type should match the fields that are returned by the Cloud Foundry API when you [get a route](https://v3-apidocs.cloudfoundry.org/version/3.185.0/#get-a-route). `AtProvider` must contain:
- Fields describing an indiviudal `API Resource`(https://v3-apidocs.cloudfoundry.org/version/3.185.0/#api-resource), including `guid`, `create_at`, and `update_at`. 
- All mutable fields, that is, parameters use to update a resource. For example, `RouteObservation` must contain all [update a route](https://v3-apidocs.cloudfoundry.org/version/3.185.0/#update-a-route), including both required parameters and optional parameters. These fields are required in order to determine if the resource is `UpToDate`.
- Any additional fields needed for special processing deletion logic. For example, if you want to prevent deletion when a route has destinations, you should include `destinations` in `RouteObservation` type.


### Generate CRD and test the new resource

As soon as you added the api definition, run  `make generate`. When the command process is completed, you will see that CRD for the resource
have been created and some additional code is generated for the resource that is expected by Crossplane.

Resource configuration is largely done. You should be abel to manually test it using a local kind cluster. Register the new CRD, prepare example for the new custom resource, and apply it to your local cluster.

```bash
> make dev-debug
> kubectl apply -f examples/route.yaml
```

### Implement the Controller

The resource is not yet functional. You need to implement the controller for the new resource. The controller is responsible for reconciling the desired state of the resource with the actual state of the resource in Cloud Foundry. The controller should be implemented in the `controllers` directory. The controller should implement the `Reconciler` interface and should be registered with the manager in the `setup.go` file.

