# Introduction

The `xpcf` tool observes *Cloud Foundry* resources and exports them as managed Crossplane resources as defined by the Cloud Foundry Crossplane provider<sup><a id="fnr.1" class="footref" href="#fn.1" role="doc-backlink">1</a></sup>.


<a id="installation"></a>

# TODO Installation


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


# Configuration

The behaviour of the `xpcf` tool can be modified by setting various configuration parameters. Some parameters are global and apply to all subcommands, while others are specific to certain subcommands.

Configuration parameters can be set through multiple methods:

-   command line flags
-   environment variables
-   configuration file

The precedence of these methods is as follows: values set in a configuration file are overridden by environment variables, and command line flags have the highest precedence.


## Command Line Flags

A command line flag may have two forms: a long form (mandatory) and a short form (optional). Flags may require a value. For *bool* type configuration parameters, the presence of the flag indicates a true value.

The following examples demonstrate different usages of CLI flags.


### Setting a bool to *true* using a short flag

The global `verbose` configuration parameter can be set using the short flag `-v`.

Example:

```bash
xpcf export -v
```


### Setting a bool to *true* using a long flag

The `verbose` parameter can also be set using the long flag `--verbose`:

```bash
xpcf export --verbose
```


### Setting a bool to *false*

A *bool* configuration parameter can be explicitly set to false using the following format:

```bash
xpcf export --verbose=false
```

Or using the short flag:

```bash
xpcf export -v=false
```


### Setting a string value

The `kind` configuration parameter of the `export` subcommand accepts string values. You can set it as follows:

```bash
xpcf export --kind space
```

Alternatively, you can use the equal sign:

```bash
xpcf export --kind=space
```


### Setting multiple strings

Some configuration parameters accept a list of strings. The `kind` parameter is one such example. You can specify multiple values by repeating the flag:

```bash
xpcf export --kind=space --kind=organization
```


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


<a id="config-file"></a>

## Configuration File

Configuration parameter values can also be set using a configuration file. The configuration file uses YAML format and must contain a YAML object where each key corresponds to a configuration parameter.

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

If no configuration file is specified, the tool searches for one in the directories specified by `XDG_CONFIG_HOME` and `HOME`, in that order.


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

The `export` and `login` subcommands are detailed in the following sections.


<a id="global-params"></a>

## Global Configuration Parameters

The global configuration parameters apply to all subcommands.


### Help

| Type                 | bool            |
|-------------------- |--------------- |
| CLI flag             | `-h` / `--help` |
| Environment variable | -               |
| Config file key      | -               |

Each subcommand provides a help option. Help can be invoked with the `-h` or `--help` CLI flag:

```bash
xpcf login --help
```


### Config

| Type                 | string            |
|-------------------- |----------------- |
| CLI flag             | `-c` / `--config` |
| Environment variable | -                 |
| Config file key      | -                 |

The `config` parameter specifies the path to the configuration file. The `export` subcommand reads configuration parameter values from the specified file.

The `login` subcommand writes the credentials to the specified configuration file. For more details about using the configuration file, refer to the [Configuration File](#config-file) section.


### Verbose

| Type                 | bool               |
|-------------------- |------------------ |
| CLI flag             | `-v` / `--verbose` |
| Environment variable | -                  |
| Config file key      | `verbose`          |

When set, the `verbose` configuration parameter enables printing of *debug-level* messages. This can be helpful for troubleshooting.


## Subcommands


<a id="login"></a>

### Login

The `login` subcommand saves the [API URL](#apiurl), [username](#username), and [password](#password) configuration values to the config file.

For examples refer to [workflows chapter](#login-workflows).


<a id="export"></a>

### Export

The `export` subcommand exports the specified resources from a *Cloud Foundry* cluster. The operation performs the following steps:

1.  Inspects the configured parameters. If the configuration parameters are insufficient to perform the requested operation, the user is prompted interactively to provide the missing values.
2.  Collects the resource configuration via the API.
3.  Converts the resource configuration according to the Crossplane provider managed resource schemas.
4.  Prints the Crossplane managed resource definitions in YAML format to the screen or to a file.


#### Authentication

There are various ways to configure the authentication parameters in `xpcf`.

The simplest approach is to use the `cf` CLI tool's<sup><a id="fnr.2" class="footref" href="#fn.2" role="doc-backlink">2</a></sup> `login` subcommand<sup><a id="fnr.3" class="footref" href="#fn.3" role="doc-backlink">3</a></sup>. This creates a configuration file with the *Cloud Foundry* API credentials, which can be reused by `xpcf` through the [`use-cf-login`](#use-cf-login) configuration parameter. See [use-cf-login](#use-cf-login) for details.

The [API URL](#apiurl), [username](#username), and [password](#password) for the *Cloud Foundry* environment can also be specified using configuration parameters via CLI flags, environment variables, or the configuration file.

The `login` subcommand can be used to update the configuration file with the [API URL](#apiurl), [username](#username), and [password](#password) values.


<a id="apiurl"></a>

##### API URL

| Type                 | string            |
|-------------------- |----------------- |
| CLI flag             | `-a` / `--apiUrl` |
| Environment variable | `API_URL`         |
| Config file key      | `apiurl`          |

This configuration parameter specifies the URL of the *Cloud Foundry* API.


<a id="username"></a>

##### Username

| Type                 | string              |
|-------------------- |------------------- |
| CLI flag             | `-u` / `--username` |
| Environment variable | `USERNAME`          |
| Config file key      | `username`          |

This configuration parameter specifies the username for authenticating with the *Cloud Foundry* API.


<a id="password"></a>

##### Password

| Type                 | string              |
|-------------------- |------------------- |
| CLI flag             | `-p` / `--password` |
| Environment variable | `PASSWORD`          |
| Config file key      | `password`          |

This configuration parameter specifies the password for authenticating with the *Cloud Foundry* API.


#### Configuration Parameters

The `export` subcommand can be configured using the [API URL](#apiurl), [username](#username), and [password](#password) configuration parameters. These parameters allow you to define the authentication details for the Cloud Foundry cluster from which resources are exported.

Alternatively, it is more convenient to use the [login](#login) subcommand, which persists these parameters in the configuration file so you don't have to specify their values for each command.

In addition to the [global configuration parameters](#global-params), the `export` subcommand also supports several subcommand-specific configuration parameters.


<a id="use-cf-login"></a>

##### Use CF Login

| Type                 | bool             |
|-------------------- |---------------- |
| CLI flag             | `--use-cf-login` |
| Environment variable | `USE_CF_LOGIN`   |
| Config file key      | `use-cf-login`   |

When set, the configuration file generated by `cf login` is used for authentication.


<a id="kind"></a>

##### Kind

| Type                 | []string        |
|-------------------- |--------------- |
| CLI flag             | `--kind` / `-k` |
| Environment variable | `KIND`          |
| Config file key      | `kind`          |

Specifies the resource kinds to export. If not set, the user is prompted interactively.

The possible values are:

-   `organization`
-   `orgrole`
-   `serviceinstance`
-   `space`
-   `spacerole`


##### Output

| Type                 | string            |
|-------------------- |----------------- |
| CLI flag             | `--output` / `-o` |
| Environment variable | `OUTPUT`          |
| Config file key      | `output`          |

The `output` parameter specifies a filename to redirect the exported YAML output to.


<a id="resolve-references"></a>

##### Resolve References

| Type                 | bool                          |
|-------------------- |----------------------------- |
| CLI flag             | `--resolve-references` / `-r` |
| Environment variable | `RESOLVE_REFERENCES`          |
| Config file key      | `resolve-references`          |

When the `resolve-references` parameter is set, cross-resource references are resolved. For example, instead of an `org` field with a GUID value, the `org.name` field is set when a resource refers to an `Organization` resource.


<a id="org"></a>

##### Org

| Type                 | []string       |
|-------------------- |-------------- |
| CLI flag             | `--org`        |
| Environment variable | -              |
| Config file key      | `organization` |

When exporting *Organization* resource kinds, the `org` parameter value specifies regular expressions that the *Organization* names must match.


<a id="space"></a>

##### Space

| Type                 | []string  |
|-------------------- |--------- |
| CLI flag             | `--space` |
| Environment variable | -         |
| Config file key      | `space`   |

When exporting *Space* resource kinds, the `space` parameter value specifies regular expressions that the *Space* names must match.


<a id="serviceinstance"></a>

##### ServiceInstance

| Type                 | []string            |
|-------------------- |------------------- |
| CLI flag             | `--serviceinstance` |
| Environment variable | -                   |
| Config file key      | `serviceinstance`   |

When exporting *ServiceInstance* resource kinds, the `serviceinstance` parameter value specifies regular expressions that the *ServiceInstance* names must match.


# Common Workflows


<a id="login-workflows"></a>

## Logging in using username and password

The [export](#export) subcommand accepts the [API URL](#apiurl), [username](#username), and [password](#password) configuration parameters for authentication. If you prefer not to specify these parameters for each `export` invocation, you can store them in a [configuration file](#config-file).

The [login](#login) subcommand simplifies this process.

You can set configuration parameter values using either CLI flags or environment variables.

Using CLI flags:

```bash
xpcf login --apiUrl 'https://test.cf.com' --username 'example-user' --password 'secret'
```

Using environment variables:

```bash
API_URL="https://test.cf.com" USERNAME="example-user" PASSWORD="secret" xpcf login
```

If any configuration value is missing, you will be prompted to enter it:

![img](vhs/login.gif "Login subcommand")


## Logging in using the `cf login` command

You can also use the `cf login` command for authentication.

For example:

```bash
cf login --sso
```

Then export resources using the [use-cf-login](#use-cf-login) configuration parameter:

```bash
xpcf export --use-cf-login
```


## Exporting interactively selected *Organization* resources

To export *Organization* resources, configure the following:

-   The credentials and API URL of the Cloud Foundry cluster ([username](#username), [password](#password), [apiurl](#apiurl))
-   The resource kind to export, which should be set to `organization` ([kind](#kind))
-   A regular expression matching the organization name(s) to export ([org](#org))

In this example, the export tool retrieves the `username`, `password`, and `apiurl` parameters from the configuration file, as they were set using the [login](#login-workflows) command.

The `kind` configuration parameter is set using a CLI flag.

The *Organization* names are selected interactively.

![img](vhs/cf-export-orgs-interactive.gif "Exporting interactively selected Organization resources")


## Exporting all *Spaces* from all *Organizations*

This example demonstrates non-interactive export of all *Spaces* from all *Organizations*. Configure the following:

-   The credentials and API URL of the Cloud Foundry cluster ([username](#username), [password](#password), [apiurl](#apiurl))
-   The resource kind to export, which should be set to `space` ([kind](#kind))
-   A regular expression matching the organization name(s) to include ([org](#org))
-   A regular expression matching the space name(s) to include ([space](#space))

In this example, the export tool retrieves the `username`, `password`, and `apiurl` parameters from the configuration file, as they were set using the [login](#login-workflows) command.

The `kind`, `org`, and `space` configuration parameters are set using CLI flags.

The regular expression `.*` matches all names.

The following command exports the required resources:

```bash
xpcf export --kind space --org '.*' --space '.*'
```


## Exporting *ServiceInstances* using *Space* name regex filters

This example shows how to export all *ServiceInstance* resources from spaces containing the word *test*. It also demonstrates how to resolve references so that they are exported by name rather than by GUID.

Configure the following:

-   The credentials and API URL of the Cloud Foundry cluster ([username](#username), [password](#password), [apiurl](#apiurl))
-   The resource kind to export, which should be set to `serviceinstance` ([kind](#kind))
-   A regular expression matching the organization name(s) to include ([org](#org))
-   A regular expression matching the space name(s) to include ([space](#space))
-   A regular expression matching the service instance name(s) to include ([serviceinstance](#serviceinstance))
-   The resolve references setting ([resolve-references](#resolve-references))

In this example, the export tool retrieves the `username`, `password`, and `apiurl` parameters from the configuration file, as they were set using the [login](#login-workflows) command.

The `kind`, `org`, `space`, `serviceinstance`, and `resolve-references` configuration parameters are set using CLI flags.

The regular expression `.*` matches all names. The regular expression `.*test.*` matches names containing the string *test*.

The following command exports the required resources:

```bash
xpcf export --kind serviceinstance --org '.*' --space '.*test.*' --serviceinstance '.*' -r
```


# Troubleshooting


# FAQ

## Footnotes

<sup><a id="fn.1" class="footnum" href="#fnr.1">1</a></sup> <https://github.com/SAP/crossplane-provider-cloudfoundry/tree/feat/export-tool>

<sup><a id="fn.2" class="footnum" href="#fnr.2">2</a></sup> <https://docs.cloudfoundry.org/cf-cli/>

<sup><a id="fn.3" class="footnum" href="#fnr.3">3</a></sup> <https://docs.cloudfoundry.org/cf-cli/getting-started.html#login>
