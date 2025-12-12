- [Introduction](#org62fd995)
- [Installation](#installation)
- [Quick Start](#org4a93068)
- [Configuration](#orgf6abd24)
- [Commands Reference](#org3b79fbc)
- [Common Workflows](#org4112ca6)
- [Troubleshooting](#orgea5b364)
- [FAQ](#org3e9505d)



<a id="org62fd995"></a>

# Introduction

The `xpcf` tool observers *Cloud Foundry* resources and export them as managed crossplane resources as defined by the Cloud Foundry Crossplane provider<sup><a id="fnr.1" class="footref" href="#fn.1" role="doc-backlink">1</a></sup>.


<a id="installation"></a>

# TODO Installation


<a id="org4a93068"></a>

# Quick Start

First, obtain a *Cloud Foundry* technical user credentials, (username and password), and a *Cloud Foundry* API endpoint.

Then, install `xpcf` following the instructions of [Installation](#installation).

Let's check that the binary can be executed with `--help` flag.

```bash
xpcf --help
```

```
Cloud Foundry exporting tool is a CLI tool for exporting existing resources as Crossplane managed resources

Usage:
  xpcf [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  export      Export Cloud Foundry resources
  help        Help about any command
  login       Logging in to Cloud Foundry cluster

Flags:
  -c, --config string   Configuration file
  -h, --help            help for xpcf
  -v, --verbose         Verbose output

Use "xpcf [command] --help" for more information about a command.
```

Next, you can persist your *Cloud Foundry* credentials using the `login` subcommand.

![img](vhs/login.gif "Login subcommand")


<a id="orgf6abd24"></a>

# Configuration

This is xpcf. Yes?


<a id="org3b79fbc"></a>

# Commands Reference


<a id="org4112ca6"></a>

# Common Workflows


<a id="orgea5b364"></a>

# Troubleshooting


<a id="org3e9505d"></a>

# FAQ

## Footnotes

<sup><a id="fn.1" class="footnum" href="#fnr.1">1</a></sup> <https://github.com/SAP/crossplane-provider-cloudfoundry/tree/feat/export-tool>
