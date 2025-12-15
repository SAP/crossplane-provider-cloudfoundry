- [Introduction](#org4cb2f9a)
- [Installation](#installation)
- [Quick Start](#org2dd5abe)
- [Configuration](#org595a70d)
  - [Command Line Flags](#org07913ad)
    - [Setting a bool to *true* using a short flag](#org9f5f169)
    - [Setting a bool to *true* using a long flag](#org4b5e49d)
    - [Setting a bool to *false*](#org99a82c1)
    - [Setting a string value](#org2566c43)
    - [Setting multiple strings](#orgcc1d20f)
- [Commands Reference](#org83f96cf)
- [Common Workflows](#orgf18055f)
- [Troubleshooting](#org4fb4aa9)
- [FAQ](#orgc37ea13)



<a id="org4cb2f9a"></a>

# Introduction

The `xpcf` tool observes *Cloud Foundry* resources and exports them as managed Crossplane resources as defined by the Cloud Foundry Crossplane provider<sup><a id="fnr.1" class="footref" href="#fn.1" role="doc-backlink">1</a></sup>.


<a id="installation"></a>

# TODO Installation


<a id="org2dd5abe"></a>

# Quick Start

First, obtain *Cloud Foundry* technical user credentials (username and password) and a *Cloud Foundry* API endpoint.

Then, install `xpcf` by following the instructions in [Installation](#installation).

Let's verify that the binary can be executed using the `--help` flag.

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

Now you can export all *Organizations* using the following command:

```bash
xpcf export --kind organization --org '.*'
```

Let's export all spaces:

```bash
xpcf export --kind space --org '.*' --space '.*'
```


<a id="org595a70d"></a>

# Configuration

The behaviour of the `xpcf` tool can be modified by setting various configuration parameters. Some parameters are global and apply to all subcommands, while others are specific to certain subcommands.

Configuration parameters can be set through multiple means:

-   command line flags
-   environment variables
-   configuration file


<a id="org07913ad"></a>

## Command Line Flags

A command line flag may have two forms: a long form (mandatory) and a short form (optional). Flags may require a value. For *bool* type configuration parameters, the presence of the flag indicates a true value.

The following examples demonstrate different usages of CLI flags.


<a id="org9f5f169"></a>

### Setting a bool to *true* using a short flag

The global `verbose` configuration parameter can be set using the short flag `-v`.

Example:

```bash
xpcf export -v
```


<a id="org4b5e49d"></a>

### Setting a bool to *true* using a long flag

The `verbose` parameter can also be set using the long flag `--verbose`:

```bash
xpcf export --verbose
```


<a id="org99a82c1"></a>

### Setting a bool to *false*

A *bool* configuration parameter can be explicitly set to false using the following format:

```bash
xpcf export --verbose=false
```

Or using the short flag:

```bash
xpcf export -v=false
```


<a id="org2566c43"></a>

### Setting a string value

The `kind` configuration parameter of the `export` subcommand accepts string values. You can set it as follows:

```bash
xpcf export --kind space
```

Alternatively, you can use the equal sign:

```bash
xpcf export --kind=space
```


<a id="orgcc1d20f"></a>

### Setting multiple strings

Some configuration parameters accept a list of strings. The `kind` parameter is one such example. You can specify multiple values by repeating the flag:

```bash
xpcf export --kind=space --kind=organization
```


<a id="org83f96cf"></a>

# Commands Reference


<a id="orgf18055f"></a>

# Common Workflows


<a id="org4fb4aa9"></a>

# Troubleshooting


<a id="orgc37ea13"></a>

# FAQ

## Footnotes

<sup><a id="fn.1" class="footnum" href="#fnr.1">1</a></sup> <https://github.com/SAP/crossplane-provider-cloudfoundry/tree/feat/export-tool>
