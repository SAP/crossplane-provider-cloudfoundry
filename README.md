# Provider CloudFoundry

`provider-cloudfoundry` is a [Crossplane](https://crossplane.io/) provider for managing your CloudFoundry resources using the
CloudFoundry V3 API. 

## Getting Started

Please check out [community guide](https://pages.github.com/SAP/docs/category/cloudfoundry)

## Developing

Run code-generation pipeline:

```console
go run cmd/generator/main.go "$PWD"
```

Run against a Kubernetes cluster:

```console
make run
```

Build, push, and install:

```console
make all
```

Build binary:

```console
make build
```

## Roadmap
We have a lot of exciting new features and improvements in our backlogs for you to expect and even contribute yourself! The major part of this roadmap will be publicly managed in github very soon.

Until then here are the top 3 buckets we are working on:

### 1. Promote Org/Space/Role management to `v1beta1`

We have completed a new implementation for these resources (currently in `v1alpha2`), which is more robust and feature complete, and uses the official Cloud Foundry [go-cfclient](https://github.com/cloudfoundry/go-cfclient). We will make a release and promote these resources to v1beta1.

### 2. Support comprehensive Service management `v1alpha2`

We received many feedbacks on service management in Cloud Foundry, including exciting new features like key rotation, `Kustomizable` service configuration, and features that support diverse comprehensive use cases in different teams. We have defined some work items in this [epic](https://github.tools.sap/kubernetes/k8s-lifecycle-management/issues/1545).

### 3. Support Apps and MTA

Many internal stakeholders want to use Crossplane to manage their applications and MTA in Cloud Foundry. We have started some PoCs and will start implementation soonest.

## Report a Bug

For filing bugs, suggesting improvements, or requesting new features, please
open an [issue](https://github.com/SAP/crossplane-provider-cloudfoundry/issues).

<a href="mailto:Daniel.Lou@sap.com">![owner](/Rep_Lou.png)</a>
