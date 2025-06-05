[![Slack](https://img.shields.io/badge/Slack-4A154B?logo=slack)](https://crossplane.slack.com/archives/C08NBTJ1J05)
![Golang](https://img.shields.io/badge/Go-1.23-informational)
[![REUSE status](https://api.reuse.software/badge/github.com/SAP/crossplane-provider-cloudfoundry)](https://api.reuse.software/info/github.com/SAP/crossplane-provider-cloudfoundry)

# Crossplane Provider for Cloud Foundry

## About this project

`crossplane-provider-cloudfoundry` is a [Crossplane](https://crossplane.io/) provider for [Cloud Foundry](https://docs.cloudfoundry.org/). The provider that is built from the source code in this repository can be installed into a Crossplane control plane and adds the following new functionality:

- Custom Resource Definitions (CRDs) that model Cloud Foundry resources (e.g. Organization, Space, Services, Applications, etc.)
- Custom Controllers to provision these resources in a Cloud Foundry deployment based on the users desired state captured in CRDs they create

## Roadmap
We have a lot of exciting new features and improvements in our backlogs for you to expect and even contribute yourself! We will publish a detailed roadmap soon.

## 📊 Installation

To install this provider in a kubernetes cluster running crossplane, you can use the provider custom resource, replacing the `<version>`placeholder with the current version of this provider (e.g. v0.3.0):

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-cloudfoundry
spec:
  package: ghcr.io/sap/crossplane-provider-cloudfoundry/crossplane/provider-cloudfoundry:<VERSION>
```

Crossplane will take care to create a deployment for this provider. Once it becomes healthy, you can configure your provider using proper credentials and start orchestrating :rocket:.

## 🔬 Developing
### Initial Setup
The provider comes with some tooling to ease a local setup for development. As initial setup you can follow these steps:
1. Clone the repository
2. Run `make submodules` to initialize the "build" submodule provided by crossplane
3. Run `make dev-debug` to create a kind cluster and install the CRDs

:warning: Please note that you are required to have [kind](https://kind.sigs.k8s.io) and [docker](https://www.docker.com/get-started/) installed on your local machine in order to run dev debug.

Those steps will leave you with a local cluster and your KUBECONFIG being configured to connect to it via e.g. [kubectl](https://kubernetes.io/docs/reference/kubectl/) or [k9s](https://k9scli.io). You can already apply manifests to that cluster at this point.

### Running the Controller
To run the controller locally, you can use the following command:
```bash
make run
```
This will compile your controller as executable and run it locally (outside of your cluster).
It will connect to your cluster using your KUBECONFIG configuration and start watching for resources.

### Cleaning up
For deleting the cluster again, run
```bash
make dev-clean
```

### E2E Tests
The provider comes with a set of end-to-end tests that can be run locally. To run them, you can use the following command:
```bash
make test-acceptance
```
This will spin up a specific kind cluster which runs the provider as docker container in it. The e2e tests will run kubectl commands against that cluster to test the provider's functionality.

Please note that when running multiple times you might want to delete the kind cluster again to avoid conflicts:
```bash
kind delete cluster <cluster-name>
```

#### Required Configuration
In order for the tests to perform successfully some configuration need to be present as environment variables:

**CF_CREDENTIALS**

User credentials for a user that is Cloud Foundry Administrator in the configured environment, structure:
```json
{
  "email": "email",
  "username": "PuserId",
  "password": "mypass"
}
```

**CF_ENVIRONMENT**

Contains the CF server URL, for example:
```
https://api.cf.eu12.hana.ondemand.com
```

## 👐 Support, Feedback, Contributing

If you have a question always feel free to reach out on our official crossplane slack channel: 

:rocket: [**#provider-sap-cloudfoundry**](https://crossplane.slack.com/archives/C08NBTJ1J05).

This project is open to feature requests/suggestions, bug reports etc. via [GitHub issues](https://github.com/SAP/crossplane-provider-cloudfoundry/issues). Contribution and feedback are encouraged and always welcome.

If you are interested in contributing, please check out our [CONTRIBUTING.md](CONTRIBUTING.md) guide and [DEVELOPER.md](DEVELOPER.md) guide.

## 🔒 Security / Disclosure
If you find any bug that may be a security problem, please follow our instructions at [in our security policy](https://github.com/SAP/crossplane-provider-cloudfoundry/security/policy) on how to report it. Please do not create GitHub issues for security-related doubts or problems.

## 🙆‍♀️ Code of Conduct

We as members, contributors, and leaders pledge to make participation in our community a harassment-free experience for everyone. By participating in this project, you agree to abide by its [Code of Conduct](https://github.com/SAP/.github/blob/main/CODE_OF_CONDUCT.md) at all times.

## 📋 Licensing

Copyright 2024 SAP SE or an SAP affiliate company and crossplane-provider-btp contributors. Please see our [LICENSE](LICENSE) for copyright and license information. Detailed information including third-party components and their licensing/copyright information is available [via the REUSE tool](https://api.reuse.software/info/github.com/SAP/crossplane-provider-btp).
