# Resource Names

```yaml
# external-name is unset
apiVersion: cloudfoundry.crossplane.io/v1alpha1
kind: Space
metadata:
  name: my-space   # name of the CR, using for cross resource referencing
  annotations:
    crossplane.io/external-name: cbad697f-cac1-48f4-9017-ac08f39dfb31 # guid of the space in the external system
spec:
  forProvider:
    name: dev-space # name of the space, unique in the org, used for filtering
    orgRef:
      name: my-org  # my-org is the metadata.name of the Org CR
status:
  atProvider:
    guid: cbad697f-cac1-48f4-9017-ac08f39dfb31
    createdAt: "2021-09-01T12:00:00Z"
    updatedAt: "2021-09-01T12:00:00Z"
```

## `metadata.name`

This is the name for the CR in the Kubernetes environment. Together with API GKV, it is unique within the namespace of the resource.

Usage:

- Identify a CR in the Kubernetes environment
- Cross Resource Referencing uses the `metadata.name` of the referenced CR.
- When possible we recommend that you use the same name as in `spec.forProvider.name`

## `spec.forProvider.name`

Some Cloud Foundry resource are named resource, e.g., Orgs, Spaces, and Service Instances. They have a `name` property in its `forProvider` spec. Normally the names  are unique in thet their respective scope. For example, for space-scoped resources, the name must be unique within the Cloud Foundry space.

When possible we recommend that you use the same names for `metadata.name` and `spec.forProvider.name`.

Usage:

- `Observe()` method may uses the `name` to query/filter the external system for the resource
- When possible we recommend that you use the same name as in `metadata.name` 


## `crossplane.io/external-name` annotation

Definition:

The `external-name` is the `guid` of the `API Resource` (https://v3-apidocs.cloudfoundry.org/version/3.185.0/#api-resource). It uniquely identifies the resource in the Cloud Foundry deployment which the provider manages. In other words, the `external-name` is used to establish the `managed` relationship between the CR in the Kubernetes environment and the actual resource in the Cloud Foundry.

Usage:
- `Create()` sets the `external-name` to the `guid` of the created resource in Cloud Foundry
- `Observe()` method uses the `external-name` to fetch the current state resource in Cloud Foundry
- `Update()` and `Delete()` methods uses the `external-name`  as the identifier to update or delete the resource in Cloud Foundry.

If an existing resource is being managed by Crossplane, the `external-name` must be set to the `guid` of the resource in the external system. In this sense, we can label all Cloud Foundry resources into two categories: managed and unmanaged. A managed resource is a resource where there exists a CR in the control plane in which the `crossplane.io/external-name` is set to the `guid` of the resource. Otherwise the resource is considered unmanaged.


Throughout the lifecycle of a CR, the `external-name` may change.  There is in general 3 cases:

1. `external-name` is unset or set to a value that is not a valid `guid` -> CR is not synced
2. `external-name` is set to a valid `guid` and the resource with that `guid` exists -> CR is synced with a managed resource in Cloud Foundry
3. `external-name` is set to a valid `guid` but the resource with that `guid` does not exists -> the managed resources in Cloud Foundry was deleted

### Special case: `external-name` is unset

With crossplane v1.16, if `external-name` is unset, Crossplane defaults it to the `metadata.name`.

If there is no valid `external-name`, the provider interprets that the CR is in an `initial` state and is not (yet) linked/pinned to any actual resource in Cloud Foundry. The controller will first query if there exists an resource that matches the `forProvider` spec of the CR, if yes, the CR will `adopt` the resource by setting the `external-name` is the `guid` of the resource. If no resource is found, a new resource will be `created` in Cloud Foundry and the `external-name` will be set to the `guid` of the newly created resource.

#Examples

- Initial state: `external-name` is unset

Note that Crossplane sets `external-name` to `my-space` by default before the first reconciliation. 

```yaml
# external-name is set to metadata.name
apiVersion: cloudfoundry.crossplane.io/v1alpha1
kind: Space
metadata:
  name: my-space
  annotations:
    crossplane.io/external-name: my-space
spec:
  forProvider:
    name: dev-space
    orgRef:
      name: my-org
```

- Synced: `external-name` is set to the `guid` of the resource. 

This can happen in two ways: the resource is created or adopted.

```yaml
apiVersion: cloudfoundry.crossplane.io/v1alpha1
kind: Space
metadata:
    name: my-space
    annotations:
        crossplane.io/external-name: 123456e8-1c34-1b34-1234-123d4567a012
spec:
    forProvider:
        name: dev-space
        orgRef:
        name: my-org  
status:
    atProvider:
        guid: 123456e8-1c34-1b34-1234-123d4567a012
        createdAt: "2024-09-01T12:00:00Z"
        updatedAt: "2024-09-01T12:00:00Z"
        name: dev-space
```

- Drift: `external-name` is set to the `guid` of the resource, but the resource is not in the desired state.

```yaml
apiVersion: cloudfoundry.crossplane.io/v1alpha1
kind: Space
metadata:
    name: my-space
    annotations:
        crossplane.io/external-name: 123456e8-1c34-1b34-1234-123d4567a012
spec:
    forProvider:
        name: dev-space
        orgRef:
        name: my-org
status: 
    atProvider:
        guid: 123456e8-1c34-1b34-1234-123d4567a012
        createdAt: "2024-09-01T12:00:00Z"
        updatedAt: "2024-09-01T12:00:00Z"
        name: test-space # not the same as spec.forProvider.name
```