![Golang](https://img.shields.io/badge/Go-1.23-informational)
[![REUSE status](https://api.reuse.software/badge/github.com/SAP/crossplane-provider-cloudfoundry)](https://api.reuse.software/info/github.com/SAP/crossplane-provider-cloudfoundry)

# Crossplane Provider for Cloud Foundry

## About this project

`crossplane-provider-cloudfoundry` is a [Crossplane](https://crossplane.io/) provider for [Cloud Foundry](https://docs.cloudfoundry.org/). The provider that is built from the source code in this repository can be installed into a Crossplane control plane and adds the following new functionality:

- Custom Resource Definitions (CRDs) that model Cloud Foundry resources (e.g. Organization, Space, Services, Applications, etc.)
- Custom Controllers to provision these resources in a Cloud Foundry deployment based on the users desired state captured in CRDs they create

## Getting Started

Please check out [community guide](https://pages.github.com/SAP/docs/category/cloudfoundry)

## Contributing

`crossplane-provider-cloudfoundry` is an SAP open-source project and we welcome contributions from the community. If you are interested in contributing, please check out our [CONTRIBUTING.md](CONTRIBUTING.md) guide and [DEVELOPER.md](DEVELOPER.md) guide.

## Roadmap
We have a lot of exciting new features and improvements in our backlogs for you to expect and even contribute yourself! The major part of this roadmap will be publicly managed in github very soon.

Until then here are the top 3 buckets we are working on:

### 1. Promote Org/Space/Role management to `v1beta1`

We have completed a new implementation for these resources (currently in `v1alpha2`), which is more robust and feature complete, and uses the official Cloud Foundry [go-cfclient](https://github.com/cloudfoundry/go-cfclient). We will make a release and promote these resources to v1beta1.

### 2. Support comprehensive Service management `v1alpha2`

We received many feedbacks on service management in Cloud Foundry, including exciting new features like key rotation, `Kustomizable` service configuration, and features that support diverse comprehensive use cases in different teams. We have defined some work items in this [epic](https://github.tools.sap/kubernetes/k8s-lifecycle-management/issues/1545).

### 3. Support Apps and MTA

Many internal stakeholders want to use Crossplane to manage their applications and MTA in Cloud Foundry. We have started some PoCs and will start implementation soonest.

## 👐 Feedbacks, Support

For feedbacks/features requests/bug reports, please
open an [issue](https://github.com/SAP/crossplane-provider-cloudfoundry/issues).

## 🔒 Security / Disclosure
If you find any bug that may be a security problem, please follow our instructions at [in our security policy](https://github.com/SAP/crossplane-provider-cloudfoundry/security/policy) on how to report it. Please do not create GitHub issues for security-related doubts or problems.

## 🙆‍♀️ Code of Conduct

We as members, contributors, and leaders pledge to make participation in our community a harassment-free experience for everyone. By participating in this project, you agree to abide by its [Code of Conduct](https://github.com/SAP/.github/blob/main/CODE_OF_CONDUCT.md) at all times.

## 📋 Licensing

Copyright 2024 SAP SE or an SAP affiliate company and crossplane-provider-btp contributors. Please see our [LICENSE](LICENSE) for copyright and license information. Detailed information including third-party components and their licensing/copyright information is available [via the REUSE tool](https://api.reuse.software/info/github.com/SAP/crossplane-provider-btp).