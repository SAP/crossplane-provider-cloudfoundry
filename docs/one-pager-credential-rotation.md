# Rotate ServiceCredentialBindings

- Status: Proposed

## 1. Context

To adhere to security best practices and compliance requirements, a mechanism is required to automatically rotate service credential bindings. 

User defines a rotation policy declaratively using a `rotation` block containing `frequency` (or `interval`) and `ttl`:

```yaml
    # Example rotation block
    rotation:
      frequency: "720h" # Rotate every 30 days
      ttl: "1440h"    # Delete keys after 60 days
```

## 2. Solution Considered

1.  **Direct Control Model**: Extending the existing `ServiceCredentialBinding` specification and its controller to natively manage the rotation lifecycle.

```yaml
apiVersion: cloudfoundry.crossplane.io/v1alpha1
kind: ServiceCredentialBinding 
metadata:
  name: my-rotating-service-key
spec:
 forProvider:
    type: key
    name: my-rotating-service-key
    serviceInstanceRef:
      name: my-database
    # The rotation policy
    rotation:
      frequency: "720h"
      ttl: "1440h"
  # The user-facing secret.   
  writeConnectionSecretToRef:
    name: my-service-credentials
    namespace: my-namespace
 ```  
   
1.  **Meta-Controller Model**: Introducing a new, higher-level CRD that handles the rotation indirectly by orchestrating `ServiceCredentialBinding` custom resources, each manages a static key in Cloud Foundry. 
  
```yaml
apiVersion: cloudfoundry.crossplane.io/v1alpha1
kind: RotatingCredentialBinding # The Parent/Meta CR
metadata:
  name: my-rotating-service-key
spec:
  # the base resource spec
  forProvider:
    type: key
    serviceInstanceRef:
        name: my-database
  # The rotation policy
  rotation:
    frequency: "720h"
    ttl: "1440h"
  # The stable, user-facing secret. 
  writeConnectionSecretToRef:
    name: my-service-credentials
    namespace: my-namespace
  
```  

We can also consider a more generic meta-controller that is capable of rotating any CRD resource. This means that `spec.forProvider` is a flexible template for any base custom resource you want to rotate. 

### Option 1: Direct Control Model

Fundamentally, there is no difference between a static key and a rotating key. A static `ServiceCredentialBinding` is simply a rotating binding with an infinite `frequency` and `ttl`.

This allows the controller to use a single, unified logic path for all ServiceCredentialBinding resources,  reducing complexity. The `rotation` block is configured directly on the `ServiceCredentialBinding` resource itself, as described in the User Configuration section.

#### 1. State Management

The state management rules are the same for all keys, whether they are configured to rotate or not

-   **`external-name` is always the active key**: The `metadata.annotations['crossplane.io/external-name']` annotation on the CR always points to the GUID of the single active key. For a static binding, this will be its one and only key. `external-name` is set/updated every time when a new service credential resources is created.

-   **The Secret is always from the active key**: The user-facing secret is always populated from the credentials of the key identified in `external-name`. There will be just one secret, as the credentials of retired key are not published.

-   `status.atProvider` is always active service credential binding plus an optional list of `retiredKeys` that keeps track of service binding resources that have been rotated out but transitionally retained due to `ttl` configuration. This changes is incremental and maintain backward compatibility.  
    ```yaml
    status:
      atProvider:
        guid: #current key
        createAt:
        retiredKeys: #all retired keys to be removed
          - guid: # retired key 1
            createdAt:
          - guid: # retired key 2
            createAt:
    ```

#### 2. The Controller's Logic

The controller's reconciliation loop does not contain an extra `if rotation-is-enabled` block. **Each rotation simply creates a new key in Cloud Foundry. Only the latest key is published to the secret.**

##### **`Observe()`

1.  Validate an **active key** (annotated by `external-name`) exists and it's valid for the given `frequency`. Otherwise, signal that `ResourceExisted: false` to trigger `Create()` method to perform a key creation/rotation.
2.  Monitor **retired keys** listed in `atProvider.retiredKeys`. If any of them exceeds `ttl`, signal that `ResourceUpToDate: false` to trigger `Update()` for clean up.

##### **`Create()`, `Update()`, and `Delete()`**

-   `Create()`: Create a new **active** key, update `external-name`.
-   `Update()`: Clean up any expired `retiredKeys` exceeding `ttl`.
-   `Delete()`: Deletes all associated keys (active and retired keys)

#### 3 Drift Detection and Correction

- Scenario 1: The *Active* Cloud Foundry Key is Manually Deleted  --> triggers a rotation immediately. Rotation cycle shift according the  new timestamp
- Scenario 2: An *Inactive* Cloud Foundry Key is Manually Deleted  --> This is "benign drift." Someone has simply done the controller's cleanup job for it.
- Scenario 3: The Secret is Manually Modified or Deleted --> Will be recreated/overwritten in the next reconciliation. 
- Scenario 4: The `external-name` annotation is tampered With -->  triggers a rotation immediately. May results in Orphan resource in CF.

### Option 2: The Meta-Controller Model 

Instead of making the existing controller smarter, it introduces a **new, separate controller** to handle the logic of rotation. **Each rotation creates a child `ServiceCredentialBinding` CR in control plane. The connection secret published in latest CR is copied over to the user-facing secret.**

#### 1. Two Types of Resources

This model involves two distinct Custom Resources:

*   **`ServiceCredentialBinding` (The Child)**: This is the existing, standard CR. Its controller knows only how to manage **one** credential key in Cloud Foundry and write it to **one** corresponding Kubernetes secret. It knows nothing about rotation.

*   **`RotatingCredentialBinding` (The Parent/Meta-CR)**: This is a brand new resource you would create. Its job is not to talk to Cloud Foundry directly, but to **create, manage, and delete** the child `ServiceCredentialBinding` resources according to a schedule.


#### 2. State Management

1. The `external-name` of the parent points to the UID of the active child CR.
2. `status.atProvider` of the parent maintains a list of all children that is alive.

#### 3. Secret Flow

There is a two-tiered secret system. 

1.  The `RotatingCredentialBinding` (parent) creates a `ServiceCredentialBinding` (child) named, for example, `my-rotating-service-key-1`.
2.  The child's controller creates the key in Cloud Foundry and writes the credentials to its **own secret**, also named `my-rotating-service-key-1`.
3.  The parent controller **watches** the child. Once the child is `Ready` and its secret exists, the parent controller **copies** the data from the child secret (`my-rotating-service-key-1`) into the stable, user-facing secret (`my-service-credentials`).
   
  
#### 4. The Meta-Controller's Logic 

The `RotatingCredentialBinding` controller's logic is purely about managing its children and copying secrets. It does not need connectivity to Cloud Foundry. 

##### **`Observe()`

1.  Validate the active child CR (annotated by `external-name`) exists,  is healthy (`synced` and `ready`), and it's valid for the given `frequency`. Otherwise, signal that `ResourceExisted: false` to trigger `Create()` method to perform a key creation/rotation.
2. If any child `status.atProvider` exceeds `ttl`, signal that `ResourceUpToDate: false` to trigger `Update()` for clean up.

##### **`Create()`, `Update()`, and `Delete()`**

-   `Create()`: Create a new **active** child CR, update `external-name`.
-   `Update()`: Clean up any child CRs exceeding `ttl`.
-   `Delete()`: Deletes everything

#### 3 Drift Detection and Correction

Drift in Cloud Foundry is managed by controller of the child CRs. 

Drift in the control plane is managed by controler of the parent CR. This is very similar to direct control model.
- Scenario 1: The *Active* Child is Manually Deleted  --> triggers a rotation immediately. Rotation cycle shift according the  new timestamp
- Scenario 2: An *Inactive* CR is Manually Deleted  --> no action needed. 
- Scenario 3: The Child Secret is Manually Modified or Deleted -->  
- Scenario 3: The User-Facing Secret is Manually Modified or Deleted -->  
- Scenario 4: The `external-name` annotation is tampered With -->  trigger a rotation immediately. May results in Orphan resource in control plane and in Cloud Foundry.
  

## 3. Comparison and Recommendation

- User Experience:
    - Direct Control Model: Users interact with a single Custom Resource (CR) and a single secret, making the process direct and intuitive.
    - Meta-Controller Model: This model introduces a split in user workflow, requiring them to choose between two different custom resource types for static vs. rotating credentials, which can be unintuitive.

- Controller logic:
    - Direct Control Model: Uses a single controller that is slightly more complex than a standard controller, as it needs to handle both credential lifecycle and rotation.
    - Meta-Controller Model: Offers better separation of concerns. The base/child resource controller can remain as-is. The meta/parent controller orchestrates rotating of multiple child resources which is as complex as the direct control model.

- Secret Handling:
    - Direct Control Model: Only one secret, regardless of the rotation policy.
    - Meta-Controller Model: Two-tier secret system adds complexity, involving at least three secrets.
  
- Extensibility and Generalization:
    - Direct Control Model: The rotation logic is tightly coupled to the `ServiceCredentialBinding` resource and its controller.
    - Meta-Controller Model: The rotation and orchestration logic is separated into a parent (meta) controller, which can be designed to work with any child resource type. The same meta-controller pattern can be reused for rotating other types of credentials or resources, simply by changing the child resource template.

**Recommendation:**

 **Direct Control Model**. The simplicity of interacting with a single CR and a single secret is a decisive advantage. The unified handling reduces cognitive load for users and is overall a more streamlined experience for managing both static and rotating credentials. 