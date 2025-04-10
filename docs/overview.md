
# What is Cloud Foundry Provider
The `crossplane-provider-cloudfoundry` is an extension of the k8s API server, using the [Kubernetes operator pattern](https://github.com/cncf/tag-app-delivery/blob/163962c4b1cd70d085107fc579e3e04c2e14d59c/operator-wg/whitepaper/Operator-WhitePaper_v1-0.md), implemented using the [Crossplane](https://crossplane.io/) framework. It extends Kubernetes capabilities by introducing Custom Resource Definitions (CRDs) for Cloud Foundry resources, such as Orgs, Space, Services, Apps, etc, and associated custom controllers to manage the full lifecycle of these resources.

- Custom Resources encodes domain-specific knowledge Cloud Foundry into Kubernetes-native APIs
- Custom Controllers encodes operational knowledge to automate the management of resource lifecycle in Cloud Foundry

## Focus
The provider focuses on a Developer's journey of developing services and applicant on a Cloud Foundry deployment, e.g., the SAP BTP Cloud Foundry environment. It is less about deploying and operating Cloud Foundry platform.  Here are some examples of the resources that the provider manages:

```bash
Orgs
 ├── OrgRoles
 ├── Spaces
 │    ├── SpaceRoles
 │    ├── Service Instances
 │    │    ├── Service Credential Bindings  
 │    ├── Apps
 │    │    ├── Route Mappings
 │    │    ├── Service Credential Bindings
 │    ├── Routes
 │    ├── Quotas
 ├── Quotas
 ├── Domains
```

The enables developer to go from the **imperative** approach using `cf cli` or UI, i.e., *telling the system what to do*, to the pure declarative API using YAML manifests to *define what the state should be*.

Imperative scripting with `cf cli`:
```bash
cf create-org my-org
cf target -o my-org
cf create-space dev
cf target -s dev
cf push my-app -m 512M --random-route
cf create-service postgres free-tier my-db
cf bind-service my-app my-db
```

Declaring the desired state with Kubernetes API:
```yaml
apiVersion: cloudfoundry.crossplane.io/v1alpha1
kind: Org
metadata:
  name: my-org
spec:
    forProvider:
        name: my-org
---
apiVersion: cloudfoundry.crossplane.io/v1alpha1
kind: Space
metadata:
  name: dev
spec:
    forProvider:
        name: dev
        orgRef:
            name: my-org
---
apiVersion: cloudfoundry.crossplane.io/v1alpha1
kind: App
metadata:
  name: my-app
spec:
    forProvider:
        name: my-app
        spaceRef:
            name: dev
        random-route: true
---
apiVersion: cloudfoundry.crossplane.io/v1alpha1
kind: ServiceInstance
metadata:
  name: my-db
spec:
    forProvider:
        name: my-db
    servicePlan: 
        offering: postgres
        plan: free-tier

---
apiVersion: cloudfoundry.crossplane.io/v1alpha1
kind: ServiceCredentialBinding
metadata:
  name: my-db-binding
spec:
    forProvider:
        type: app
        appRef:
            name: my-app
        serviceInstanceRef:
            name: my-db

```

# Project structure

The repository is structured as follows: (showing the developer relevant directories)

```bash
crossplane-provider-cloudfoundry/
│── apis/               # API definitions
│   ├── resources/        
│   │   ├── v1alpha1/   # v1alpha1 API definitions
│   │   │   ├── route_types.go   
│   │   │   ├── zz_generated.deepcopy.go   
│   │   │   ├── zz_generated.managed.go  
│   │   │   ├── zz_generated.resolvers.go  
│   ├── generate.go
│── package/           
│   ├── crds/           # Generated Custom Resource Definitions
│── internal/
│   ├── clients/        # Implement client interfaces using go-cfclient and mock client
│   │   ├── route/      
│   ├── controller/     # User dependency injection for client interface
│   │   ├── route/
│   │   │   ├── controller.go  
│   ├── clients/       
│── test/               # e2e tests
│   ├── e2e/           
│── go.mod            
│── go.sum            
│── Makefile          
│── README.md         
```
