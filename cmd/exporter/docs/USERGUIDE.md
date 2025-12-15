- [Introduction](#org6dc8a15)
- [Installation](#installation)
- [Quick Start](#orge909076)
- [Configuration](#orge4c5aea)
  - [Command Line Flags](#orgefa3d8d)
    - [Setting a bool to *true* using a short flag](#org34e8950)
    - [Setting a bool to *true* using a long flag](#org18e5fcc)
    - [Setting a bool to *false*](#org06d745c)
    - [Setting a string value](#orgd16478d)
    - [Setting multiple strings](#org6368e7f)
  - [Environment Variables](#org24285b9)
  - [Configuration File](#orge3fd5f5)
- [Commands Reference](#orgd7ce56c)
- [Common Workflows](#orgb322215)
- [Troubleshooting](#org997ec09)
- [FAQ](#org320b874)



<a id="org6dc8a15"></a>

# Introduction

The `xpcf` tool observes *Cloud Foundry* resources and exports them as managed Crossplane resources as defined by the Cloud Foundry Crossplane provider<sup><a id="fnr.1" class="footref" href="#fn.1" role="doc-backlink">1</a></sup>.


<a id="installation"></a>

# TODO Installation


<a id="orge909076"></a>

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


<a id="orge4c5aea"></a>

# Configuration

The behaviour of the `xpcf` tool can be modified by setting various configuration parameters. Some parameters are global and apply to all subcommands, while others are specific to certain subcommands.

Configuration parameters can be set through multiple means:

-   command line flags
-   environment variables
-   configuration file

The precedence of these methods is as follows: values set in a configuration file are overridden by environment variables, and command line flags have the highest precedence.


<a id="orgefa3d8d"></a>

## Command Line Flags

A command line flag may have two forms: a long form (mandatory) and a short form (optional). Flags may require a value. For *bool* type configuration parameters, the presence of the flag indicates a true value.

The following examples demonstrate different usages of CLI flags.


<a id="org34e8950"></a>

### Setting a bool to *true* using a short flag

The global `verbose` configuration parameter can be set using the short flag `-v`.

Example:

```bash
xpcf export -v
```


<a id="org18e5fcc"></a>

### Setting a bool to *true* using a long flag

The `verbose` parameter can also be set using the long flag `--verbose`:

```bash
xpcf export --verbose
```


<a id="org06d745c"></a>

### Setting a bool to *false*

A *bool* configuration parameter can be explicitly set to false using the following format:

```bash
xpcf export --verbose=false
```

Or using the short flag:

```bash
xpcf export -v=false
```


<a id="orgd16478d"></a>

### Setting a string value

The `kind` configuration parameter of the `export` subcommand accepts string values. You can set it as follows:

```bash
xpcf export --kind space
```

Alternatively, you can use the equal sign:

```bash
xpcf export --kind=space
```


<a id="org6368e7f"></a>

### Setting multiple strings

Some configuration parameters accept a list of strings. The `kind` parameter is one such example. You can specify multiple values by repeating the flag:

```bash
xpcf export --kind=space --kind=organization
```


<a id="org24285b9"></a>

## Environment Variables

Certain configuration parameters can be set using environment variables.

For a parameter of type *bool*, set the value of the environment variable to `"1"`:

```bash
VERBOSE=1 xpcf export
```

If the configuration parameter is of type *list of strings*, separate the list elements with *space* characters:

```bash
KIND="organization space" xpcf export
```


<a id="orge3fd5f5"></a>

## Configuration File

Configuration parameter values can also be set using a configuration file. The configuration file uses YAML format and must contain a YAML object where each key is a configuration parameter.

An example configuration file `example-config.yaml` is shown below:

```yaml
verbose: true
kind:
  - organization
  - space
```

To use a configuration file, specify it with the `-c` or `--config` command line flag:

```bash
xpcf export --config example-config.yaml
```

If no configuration file is specified, the tool searches for one ine the directories specified by `XDG_CONFIG_HOME` and `HOME`, in that order.


<a id="orgd7ce56c"></a>

# Commands Reference


<a id="orgb322215"></a>

# Common Workflows


<a id="org997ec09"></a>

# Troubleshooting


<a id="org320b874"></a>

# FAQ

## Footnotes

<sup><a id="fn.1" class="footnum" href="#fnr.1">1</a></sup> <https://github.com/SAP/crossplane-provider-cloudfoundry/tree/feat/export-tool>
