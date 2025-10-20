# Development Setup

If you want to contribute to this Crossplane cloudfoundry provider, be aware of the contribution guideline.

## Local Setup

Ensure you have the following tools installed:
- git
- go
- golangci-lint
- make
- docker
- helm
- kind
- kubectl
- k9s

## Developing

Clone the repository:

```console
git clone https://github.com/SAP/crossplane-provider-cloudfoundry.git
cd crossplane-provider-cloudfoundry
```


checkout the branch you want to work on:

```console
git checkout -b <branch-name>
```

init and update submodules:

```console
make submodules
```

Configure connection details to your local development environment in `examples/providerconfig/` directory. You can use the `examples/providerconfig/providerconfig.yaml` and `examples/providerconfig/secret.yaml.tmpl` as a template.


You can now start developing the provider.

## Local Build

Run code-generation pipeline:

```console
make generate
```

To test the provider with a local kind cluster, first run:

```console
make dev-debug
```

This create a dev cluster, install crossplane, and install the provider. You can use `k9s` to check the status of the cluster. You can also use `kubectl` to apply and test custom resources.

Then, in another terminal, run

```console
make run
```

This runs the controller locally.

## Local Kind Build

If you want to run the controller component and the CrossPlane controller in the same local kind cluster, run:

```console
make local-deploy
```

## Local e2e test

Build, push, and install:

```console
make all
```

Build binary:

```console
make build
```

To start the local e2e test:

```console
export CF_ENVIRONMENT=https://cf-environment.com
export CF_CREDENTIALS={"user": "someuserEmail", "email": "useremailaddress@s.e", "password": "supersecretPassword"}
make test-acceptance
```

## Report a Bug

For filing bugs, suggesting improvements, or requesting new features, please
open an [issue](https://github.com/SAP/crossplane-provider-cloudfoundry/issues).

<a href="mailto:Daniel.Lou@example.com">![owner](/Rep_Lou.png)</a>
