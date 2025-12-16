- [Introduction](#org026e14b)
- [Installation](#installation)
- [Quick Start](#orgeb19510)
- [Configuration](#org170c8f1)
  - [Command Line Flags](#org6a1c4e3)
    - [Setting a bool to *true* using a short flag](#orgf25aae7)
    - [Setting a bool to *true* using a long flag](#org4f4d0e2)
    - [Setting a bool to *false*](#org70fff88)
    - [Setting a string value](#org67a8855)
    - [Setting multiple strings](#orge32af47)
  - [Environment Variables](#org4326320)
  - [Configuration File](#org8572530)
- [Commands Reference](#orge4956c1)
  - [Global configuration parameters](#org47eca7a)
    - [Help](#orgf4bf757)
    - [Config](#org55b4d73)
- [Common Workflows](#org3897ca8)
- [Troubleshooting](#org8e2e6a0)
- [FAQ](#orge58a718)



<a id="org026e14b"></a>

# Introduction

The `xpcf` tool observes *Cloud Foundry* resources and exports them as managed Crossplane resources as defined by the Cloud Foundry Crossplane provider<sup><a id="fnr.1" class="footref" href="#fn.1" role="doc-backlink">1</a></sup>.


<a id="installation"></a>

# TODO Installation


<a id="orgeb19510"></a>

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


<a id="org170c8f1"></a>

# Configuration

The behaviour of the `xpcf` tool can be modified by setting various configuration parameters. Some parameters are global and apply to all subcommands, while others are specific to certain subcommands.

Configuration parameters can be set through multiple means:

-   command line flags
-   environment variables
-   configuration file

The precedence of these methods is as follows: values set in a configuration file are overridden by environment variables, and command line flags have the highest precedence.


<a id="org6a1c4e3"></a>

## Command Line Flags

A command line flag may have two forms: a long form (mandatory) and a short form (optional). Flags may require a value. For *bool* type configuration parameters, the presence of the flag indicates a true value.

The following examples demonstrate different usages of CLI flags.


<a id="orgf25aae7"></a>

### Setting a bool to *true* using a short flag

The global `verbose` configuration parameter can be set using the short flag `-v`.

Example:

```bash
xpcf export -v
```


<a id="org4f4d0e2"></a>

### Setting a bool to *true* using a long flag

The `verbose` parameter can also be set using the long flag `--verbose`:

```bash
xpcf export --verbose
```


<a id="org70fff88"></a>

### Setting a bool to *false*

A *bool* configuration parameter can be explicitly set to false using the following format:

```bash
xpcf export --verbose=false
```

Or using the short flag:

```bash
xpcf export -v=false
```


<a id="org67a8855"></a>

### Setting a string value

The `kind` configuration parameter of the `export` subcommand accepts string values. You can set it as follows:

```bash
xpcf export --kind space
```

Alternatively, you can use the equal sign:

```bash
xpcf export --kind=space
```


<a id="orge32af47"></a>

### Setting multiple strings

Some configuration parameters accept a list of strings. The `kind` parameter is one such example. You can specify multiple values by repeating the flag:

```bash
xpcf export --kind=space --kind=organization
```


<a id="org4326320"></a>

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


<a id="org8572530"></a>

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


<a id="orge4956c1"></a>

# Commands Reference

The `xpcf` tool supports four subcommands:

-   `completion`
-   `export`
-   `help`
-   `login`

The `completion` subcommand generates autocompletion scripts for various shells:

```bash
xpcf completion --help
```

The `help` subcommand prints a generic help message about subcommands and global CLI flags.

The `export` and `login` subcommands are detailed in the upcoming sections.


<a id="org47eca7a"></a>

## Global configuration parameters

The global configuration parameters are valid for each subcommand.


<a id="orgf4bf757"></a>

### Help

| Configuration        | Value           |
|-------------------- |--------------- |
| CLI flag             | `-h` / `--help` |
| Environment variable | -               |
| Config file key      | -               |

Each subcommand comes with a help configuration parameter. Help can be invoked with the `-h` or `--help` CLI flag:

```bash
xpcf login --help
```


<a id="org55b4d73"></a>

### Config


<a id="org3897ca8"></a>

# Common Workflows


<a id="org8e2e6a0"></a>

# Troubleshooting


<a id="orge58a718"></a>

# FAQ

## Footnotes

<sup><a id="fn.1" class="footnum" href="#fnr.1">1</a></sup> <https://github.com/SAP/crossplane-provider-cloudfoundry/tree/feat/export-tool>
