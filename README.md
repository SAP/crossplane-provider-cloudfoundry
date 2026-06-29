[![Slack](https://img.shields.io/badge/Slack-4A154B?logo=slack)](https://crossplane.slack.com/archives/C08NBTJ1J05)
![Golang](https://img.shields.io/badge/Go-1.23-informational)
[![REUSE status](https://api.reuse.software/badge/github.com/SAP/crossplane-provider-cloudfoundry)](https://api.reuse.software/info/github.com/SAP/crossplane-provider-cloudfoundry)

# Crossplane Provider for Cloud Foundry

> **Manage Cloud Foundry resources the GitOps way** — declarative, version-controlled, and reconciled continuously by Crossplane.

`crossplane-provider-cloudfoundry` lets you define Cloud Foundry resources (Orgs, Spaces, Services, Applications, and more) as Kubernetes custom resources. Crossplane takes care of the rest: provisioning, drift detection, and reconciliation — no scripts, no manual clicks.

---

## 🌍 Community

We're building this in the open and we'd love your involvement — whether you're using the provider in production, experimenting with it, or just curious about how it works.

---

### 📞 Monthly Community Call

We hold a community call on the **last Wednesday of every month at 4 pm CET**.

- 👀 See what everyone's been working on
- 💡 Share ideas and feedback
- ❓ Ask questions — no question is too basic
- 🔬 Dive into technical details together

🔗 **Join:** : [Click Here](https://teams.microsoft.com/meet/38649588898748?p=ctg4uzLFVdSWRU2ZJQ)

🎬 **Can't make it?** Recordings are shared after each call.

---

### 💬 Chat with us

Join us on Slack: [**#provider-sap-cloudfoundry**](https://crossplane.slack.com/archives/C08NBTJ1J05) — for questions, ideas, or just to say hi.

---

## 🗺️ Roadmap

We have a growing backlog of features and improvements. You can follow along — and pick something up! — on our [GitHub Issues](https://github.com/SAP/crossplane-provider-cloudfoundry/issues) and [Discussions](https://github.com/SAP/crossplane-provider-cloudfoundry/discussions).

---

## 👐 Support, Feedback

Got a question or found an issue? We'd love to hear from you:

- 🐛 **Open a GitHub issue:** [github.com/SAP/crossplane-provider-cloudfoundry/issues/new/choose](https://github.com/SAP/crossplane-provider-cloudfoundry/issues/new/choose)
- 💬 **Chat on Slack:** [**#provider-sap-cloudfoundry**](https://crossplane.slack.com/archives/C08NBTJ1J05)
- 📞 **Join the monthly community call** — last Wednesday of every month at 4 pm CET

---

## 📦 Installation

> ⚠️ **Crossplane v2 is not yet supported but coming soon.** Please use **Crossplane v1** for now.

To install the provider into a Kubernetes cluster running Crossplane, apply the following resource — replacing `<VERSION>` with the [latest release](https://github.com/SAP/crossplane-provider-cloudfoundry/releases):

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-cloudfoundry
spec:
  package: ghcr.io/sap/crossplane-provider-cloudfoundry/crossplane/provider-cloudfoundry:<VERSION>
```

Crossplane will create a deployment for the provider. Once it's healthy, configure your credentials and start orchestrating. 🚀

---

## 🔬 Developing

### Initial Setup

The provider includes tooling to get you up and running locally quickly.

**Prerequisites:** [kind](https://kind.sigs.k8s.io) and [Docker](https://www.docker.com/get-started/) must be installed.

```bash
# 1. Clone the repo
git clone https://github.com/SAP/crossplane-provider-cloudfoundry

# 2. Initialize the build submodule
make submodules

# 3. Spin up a local kind cluster with CRDs installed
make dev-debug
```

This leaves you with a local cluster and your `KUBECONFIG` pointed at it — ready for `kubectl` or [k9s](https://k9scli.io).

### Running the Controller

```bash
make run
```

Compiles and runs the controller locally (outside the cluster), watching for resources via your `KUBECONFIG`.

### Cleaning Up

```bash
make dev-clean
```

### E2E Tests

```bash
make test-acceptance
```

Spins up a kind cluster, runs the provider as a container inside it, and fires `kubectl` commands to validate behavior end-to-end.

If you run tests multiple times, clean up the kind cluster first to avoid conflicts:

```bash
kind delete cluster <cluster-name>
```

### Upgrade Tests

See the [Upgrade Tests README](./docs/upgrade-testing.md) for details.

#### Required Environment Variables

| Variable | Description |
|---|---|
| `CF_CREDENTIALS` | CF admin user credentials as JSON: `{"email": "...", "username": "...", "password": "..."}` |
| `CF_ENVIRONMENT` | CF API URL, e.g. `https://api.cf.eu12.hana.ondemand.com` |

---

## 🛠️ Export CLI

The provider ships an export CLI that generates managed resource definitions from an existing Cloud Foundry cluster's configuration.

→ See the [User Guide](cmd/exporter/docs/USERGUIDE.md) for details.

---

## 🤝 Contributing

Contributions are very welcome — from bug reports to new features to docs improvements. Here's how to get involved:

- 🐛 **Found a bug?** [Open an issue](https://github.com/SAP/crossplane-provider-cloudfoundry/issues)
- 💡 **Have an idea?** Start a [Discussion](https://github.com/SAP/crossplane-provider-cloudfoundry/discussions) or drop by the community call
- 🛠️ **Want to contribute code?** Check out the [CONTRIBUTING.md](CONTRIBUTING.md) and [DEVELOPER.md](DEVELOPER.md) guides

Not sure where to start? Come chat on [Slack](https://crossplane.slack.com/archives/C08NBTJ1J05) — we're happy to help you find something.

---

## 🔒 Security

If you discover a potential security vulnerability, please follow our [security policy](https://github.com/SAP/crossplane-provider-cloudfoundry/security/policy) — **do not open a public GitHub issue** for security concerns.

---

## 🙆‍♀️ Code of Conduct

We're committed to a welcoming, harassment-free community for everyone. By participating in this project, you agree to our [Code of Conduct](https://github.com/SAP/.github/blob/main/CODE_OF_CONDUCT.md).

---

## 📋 Licensing

Copyright 2024 SAP SE or an SAP affiliate company and crossplane-provider-cloudfoundry contributors. See [LICENSE](LICENSE) for details. Third-party component licensing is available via the [REUSE tool](https://api.reuse.software/info/github.com/SAP/crossplane-provider-btp).
