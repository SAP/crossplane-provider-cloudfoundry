- [Introduction](#org5e496f7)
- [Examples](#org951a18d)
  - [The simplest CLI tool](#org15a6542)
  - [Exporting](#org6514a3e)
    - [Basic export subcommand](#orgd1ac689)
    - [Exporting a resource](#org716c76d)
    - [Displaying warnings](#org0c028ba)
    - [Exporting commented out resources](#commented-export)
  - [Errors with attributes](#erratt-example)
  - [Widgets](#orgb51fd22)
    - [TextInput widget](#org8876149)
    - [MultiInput widget](#orga942dec)
  - [Configuration parameters](#org996445e)
    - [Global configuration parameters](#orgbaaa2af)
      - [Verbose logging](#verbose)
    - [Configuration parameters of the export subcommand](#org0ebf2aa)
    - [Bool configuration parameter](#org2139062)
    - [String configuration parameter](#orgfac3d5b)
    - [String slice configuration parameter](#orgfd67f35)
      - [Without possible values](#org506ac65)
      - [With static possible values](#orgc3863da)
      - [With dynamic possible values](#orgd4baeba)
    - [Configuration file](#config-file)
  - [Parsing and sanitizing](#orgc358bd5)
    - [Sanitizer rules](#org50009f6)
      - [RFC1035Subdomain](#org0460922)
      - [RFC1035LowerSubdomain](#org8cf7fab)
      - [RFC1035SubdomainRelaxed](#org2ee447c)
      - [RFC1035LowerSubdomainRelaxed](#orgd6f79b2)



<a id="org5e496f7"></a>

# Introduction

`xp-clifford` (Crossplane CLI Framework for Resource Data Extraction) is a Go module that facilitates the development of CLI tools for exporting definitions of external resources in the format of specific Crossplane provider managed resource definitions.

The resource definitions can then be imported into Crossplane using the [standard import procedure](https://docs.crossplane.io/v2.1/guides/import-existing-resources/). It is recommended to check the generated definitions for comments, before doing the import. See also [Exporting commented out resources](#commented-export).


<a id="org951a18d"></a>

# Examples

These examples demonstrate the basic features of `xp-clifford` and build progressively on one another.


<a id="org15a6542"></a>

## The simplest CLI tool

The simplest CLI tool you can create using `xp-clifford` looks like this:

```go
package main

import (
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	_ "github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
)

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	cli.Execute()
}
```

Let's examine the `import` section.

```go
import (
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	_ "github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
)
```

Two packages must be imported:

-   `github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli`
-   `github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export`

The `cli/export` package is imported for side effects only.

The `main` function looks like this:

```go
func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	cli.Execute()
}
```

The `Configuration` variable from the `cli` package is used to set specific parameters for the built CLI tool. Here we set the `ShortName` and `ObservedSystem` fields.

These fields have the following meanings:

-   **ShortName:** The abbreviated name of the observed system without spaces, such as "cf" for the CloudFoundry provider
-   **ObservedSystem:** The full name of the external system, which may contain spaces, such as "Cloud Foundry"

At the end of the `main` function, we invoke the `Execute` function from the `cli` package to start the CLI.

When we run this basic example, it generates the following output:

```sh
go run ./examples/basic/main.go
```

```
test system exporting tool is a CLI tool for exporting existing resources as Crossplane managed resources

Usage:
  test-exporter [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  export      Export test system resources
  help        Help about any command

Flags:
  -c, --config string   Configuration file
  -h, --help            help for test-exporter
  -v, --verbose         Verbose output

Use "test-exporter [command] --help" for more information about a command.
```

If you try running the CLI tool with the export subcommand, you get an **error** message.

```sh
go run ./examples/basic/main.go export
```

    ERRO export subcommand is not set


<a id="org6514a3e"></a>

## Exporting


<a id="orgd1ac689"></a>

### Basic export subcommand

The `export` subcommand is mandatory, but you are responsible for implementing the code that executes when it is invoked.

The code must be defined as a function with the following signature:

```go
func(ctx context.Context, events export.EventHandler) error
```

The `ctx` parameter can be used to handle interruptions, such as when the user presses *Ctrl-C*. In such cases, the `Done()` channel of the context is closed.

The `events` parameter from the `export` package provides three methods for communicating progress to the CLI framework:

-   **Warn:** Indicates a recoverable error that does not terminate the export operation.
-   **Resource:** Indicates a processed managed resource to be printed or stored by the export operation.
-   **Stop:** Indicates that exporting has finished. No more `Warn` or `Resource` calls should be made after `Stop`.

A fatal error can be indicated by returning a non-nil error value.

A simple implementation of an export logic function looks like this:

```go
func exportLogic(_ context.Context, events export.EventHandler) error {
	slog.Info("export command invoked")
	events.Stop()
	return nil
}
```

This implementation prints a log message, stops the event handler, and returns a `nil` error value.

You can configure the business logic function using the `SetCommand` function from the `export` package:

```go
export.SetCommand(exportLogic)
```

A complete example is:

```go
package main

import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
)

func exportLogic(_ context.Context, events export.EventHandler) error {
	slog.Info("export command invoked")
	events.Stop()
	return nil
}

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	export.SetCommand(exportLogic)
	cli.Execute()
}
```

To invoke the `export` subcommand:

```sh
go run ./examples/export/main.go export
```

    INFO export command invoked


<a id="org716c76d"></a>

### Exporting a resource

In the previous example, we created a proper `export` subcommand, but didn't actually export any resources.

To export a resource, use the `Resource` method of the `EventHandler` type:

```go
Resource(res resource.Object) // Object interface defined in
                              // github.com/crossplane/crossplane-runtime/pkg/resource
```

This method accepts a `resource.Object`, an interface implemented by all Crossplane resources.

Let's update our `exportLogic` function to export a single resource. For simplicity, we'll use the `Unstructured` type from `k8s.io/apimachinery/pkg/apis/meta/v1/unstructured`, which implements the `resource.Object` interface:

```go
func exportLogic(_ context.Context, events export.EventHandler) error {
	slog.Info("export command invoked")

	res := &unstructured.Unstructured{
	  Object: map[string]interface{}{
	      "user": "test-user",
	      "password": "secret",
	  },
	}
	events.Resource(res)

	events.Stop()
	return nil
}
```

The complete example now looks like this:

```go
package main

import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func exportLogic(_ context.Context, events export.EventHandler) error {
	slog.Info("export command invoked")

	res := &unstructured.Unstructured{
	  Object: map[string]interface{}{
	      "user": "test-user",
	      "password": "secret",
	  },
	}
	events.Resource(res)

	events.Stop()
	return nil
}

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	export.SetCommand(exportLogic)
	cli.Execute()
}
```

Running this example produces the following output:

```sh
go run ./examples/exportsingle/main.go export
```

    INFO export command invoked
    
    
        ---
        password: secret
        user: test-user
        ...

The exported resource is printed to the console. You can redirect the output to a file using the `-o` flag:

```sh
go run ./examples/exportsingle/main.go export -o output.yaml
```

    INFO export command invoked
    INFO Writing output to file output=output.yaml

The `output.yaml` file contains the exported resource object:

```sh
cat output.yaml
```

    ---
    password: secret
    user: test-user
    ...


<a id="org0c028ba"></a>

### Displaying warnings

During the processing and conversion of external resources, the export logic may encounter unexpected situations such as unstable network connections, authentication issues, or unknown resource configurations.

These events should not halt the resource export process, but they must be reported to the user.

You can report warnings using the `Warn` method of the `EventHandler` type:

```go
Warn(err error)
```

The `Warn` method supports `erratt.Error` types. The `erratt.Error` type is demonstrated in [2.3](#erratt-example).

Let's add a warning message to our `exportLogic` function:

```go
func exportLogic(_ context.Context, events export.EventHandler) error {
	slog.Info("export command invoked")

	events.Warn(errors.New("generating test resource"))

	res := &unstructured.Unstructured{
	  Object: map[string]interface{}{
	      "user": "test-user-with-warning",
	      "password": "secret",
	  },
	}
	events.Resource(res)

	events.Stop()
	return nil
}
```

The complete example now looks like this:

```go
package main

import (
	"context"
	"errors"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func exportLogic(_ context.Context, events export.EventHandler) error {
	slog.Info("export command invoked")

	events.Warn(errors.New("generating test resource"))

	res := &unstructured.Unstructured{
	  Object: map[string]interface{}{
	      "user": "test-user-with-warning",
	      "password": "secret",
	  },
	}
	events.Resource(res)

	events.Stop()
	return nil
}

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	export.SetCommand(exportLogic)
	cli.Execute()
}
```

Running this example displays the warning message in the logs:

```sh
go run ./examples/exportwarn/main.go export
```

    INFO export command invoked
    WARN generating test resource
    
    
        ---
        password: secret
        user: test-user-with-warning
        ...

When redirecting the output to a file, the warning appears on screen but not in the file:

```sh
go run ./examples/exportwarn/main.go export -o output.yaml
```

    INFO export command invoked
    WARN generating test resource
    INFO Writing output to file output=output.yaml

```sh
cat output.yaml
```

    ---
    password: secret
    user: test-user-with-warning
    ...


<a id="commented-export"></a>

### Exporting commented out resources

During the export process, problems may prevent generation of valid managed resource definitions, or the definitions produced may be unsafe to apply.

You have two options for handling problematic resources: omit them from the output entirely, or include them but commented out. Commenting out invalid or unsafe resource definitions ensures users won't encounter problems when applying the export tool output.

`xp-clifford` comments out resources that implement the `yaml.CommentedYAML` interface, which defines a single method:

```go
type CommentedYAML interface {
	Comment() (string, bool)
}
```

The `bool` return value indicates whether the managed resource should be commented out. The `string` return value provides a message that will be printed as part of the comment.

Since Crossplane managed resources don't typically implement the `CommentedYAML` interface, you can wrap them to add this functionality.

The `yaml.NewResourceWithComment` function handles this wrapping for you:

```go
func NewResourceWithComment(res resource.Object) *yaml.ResourceWithComment
```

The `*yaml.ResourceWithComment` type wraps `res` and implements the `yaml.CommentedYAML` interface. It also provides helper methods:

-   **SetComment:** sets the comment string
-   **AddComment:** appends to the comment string

The following example demonstrates the commenting feature:

```go
func exportLogic(_ context.Context, events export.EventHandler) error {
	slog.Info("export command invoked")

	res := &unstructured.Unstructured{
	  Object: map[string]interface{}{
	      "user": "test-user-commented",
	      "password": "secret",
	  },
	}

	commentedResource := yaml.NewResourceWithComment(res)
	commentedResource.SetComment("don't deploy it, this is a test resource!")
	events.Resource(commentedResource)

	events.Stop()
	return nil
}
```

Here is the complete example:

```go
package main

import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/yaml"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func exportLogic(_ context.Context, events export.EventHandler) error {
	slog.Info("export command invoked")

	res := &unstructured.Unstructured{
	  Object: map[string]interface{}{
	      "user": "test-user-commented",
	      "password": "secret",
	  },
	}

	commentedResource := yaml.NewResourceWithComment(res)
	commentedResource.SetComment("don't deploy it, this is a test resource!")
	events.Resource(commentedResource)

	events.Stop()
	return nil
}

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	export.SetCommand(exportLogic)
	cli.Execute()
}
```

Running this example displays the commented resource with its comment message:

```sh
go run ./examples/exportcomment/main.go export
```

```
INFO export command invoked


    #
    # don't deploy it, this is a test resource!
    #
    # ---
    # password: secret
    # user: test-user-commented
    # ...

```

This works equally well when redirecting output to a file using the `-o` flag.


<a id="erratt-example"></a>

## Errors with attributes

The `erratt` package implements a new `error` type designed for efficient use with the `Warn` method of `EventHandler`.

The `erratt.Error` type implements the standard Go `error` interface. Additionally, it can be extended with `slog` package compatible key-value pairs used for structured logging. The `erratt.Error` type also supports wrapping Go `error` values. When an `erratt.Error` is wrapped, its attributes are preserved.

You can create a simple `erratt.Error` using the `erratt.New` function:

```go
err := erratt.New("something went wrong")
errWithAttrs1 := erratt.New("error opening file", "filename", filename)
errWithAttrs2 := erratt.New("authentication failed", "username", user, "password", pass)
```

In this example, `errWithAttrs1` and `errWithAttrs2` include additional attributes.

You can wrap an existing `error` value using the `erratt.Errorf` function:

```go
err := callFunction()
errWrapped := erratt.Errorf("unexpected error occurred: %w", err)
```

You can extend an `erratt.Error` value with attributes using the `With` method:

```go
err := connectToServer(url, username, password)
errWrapped := erratt.Errorf("cannot connect to server: %w", err).
	With("url", url, "username", username, "password", password)
```

For a complete example, consider two functions that return `erratt.Error` values and demonstrate wrapping:

```go
func auth() erratt.Error {
	return erratt.New("authentication failure",
		"username", "test-user",
		"password", "test-password",
	)
}

func connect() erratt.Error {
	err := auth()
	if err != nil {
		return erratt.Errorf("connect failed: %w", err).
			With("url", "https://example.com")
	}
	return nil
}
```

The `auth` function returns an `erratt.Error` value with username and password attributes.

The `exportLogic` function calls `connect` and handles the error:

```go
func exportLogic(_ context.Context, events export.EventHandler) error {
	slog.Info("export command invoked")

	err := connect()

	events.Stop()
	return err
}
```

Here is the complete example:

```go
package main

import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/erratt"
)

func auth() erratt.Error {
	return erratt.New("authentication failure",
		"username", "test-user",
		"password", "test-password",
	)
}

func connect() erratt.Error {
	err := auth()
	if err != nil {
		return erratt.Errorf("connect failed: %w", err).
			With("url", "https://example.com")
	}
	return nil
}

func exportLogic(_ context.Context, events export.EventHandler) error {
	slog.Info("export command invoked")

	err := connect()

	events.Stop()
	return err
}

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	export.SetCommand(exportLogic)
	cli.Execute()
}
```

Running this code produces the following output:

```sh
go run ./examples/erratt/main.go export
```

    INFO export command invoked
    ERRO connect failed: authentication failure url=https://example.com username=test-user password=test-password

The error message appears on the console with all attributes displayed.

The `EventHandler.Warn` method handles `erratt.Error` values in the same manner.


<a id="orgb51fd22"></a>

## Widgets

`xp-clifford` provides several CLI widgets to facilitate the interaction with the user.

Note that for the widgets to run, the CLI tool must be executed in an interactive terminal. This is not always the case by default, when running or debugging an application within an IDE (like GoLand) using a Run Configuration. In such cases, make sure to configure the Run Configuration appropriately. Specifically for [GoLand](https://www.jetbrains.com/help/go/run-debug-configuration.html) it can be done by selecting `Emulate terminal in output console`.


<a id="org8876149"></a>

### TextInput widget

The TextInput widget prompts the user for a single line of text. Create a TextInput widget using the `TextInput` function from the `widget` package.

```go
func TextInput(ctx context.Context, title, placeholder string, sensitive bool) (string, error)
```

Parameters:

-   **ctx:** Go context for handling Ctrl-C interrupts or timeouts
-   **title:** The prompt question displayed to the user
-   **placeholder:** Placeholder text shown when the input is empty
-   **sensitive:** When true, masks typed characters (useful for passwords)

The following example demonstrates an `exportLogic` function that prompts for a username and password:

```go
func exportLogic(ctx context.Context, events export.EventHandler) error {
	slog.Info("export command invoked")

	username, err := widget.TextInput(ctx, "Username", "anonymous", false)
	if err != nil {
		return err
	}

	password, err := widget.TextInput(ctx, "Password", "", true)
	if err != nil {
		return err
	}

	slog.Info("data acquired",
		"username", username,
		"password", password,
	)

	events.Stop()
	return err
}
```

Complete example:

```go
package main

import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/widget"
)

func exportLogic(ctx context.Context, events export.EventHandler) error {
	slog.Info("export command invoked")

	username, err := widget.TextInput(ctx, "Username", "anonymous", false)
	if err != nil {
		return err
	}

	password, err := widget.TextInput(ctx, "Password", "", true)
	if err != nil {
		return err
	}

	slog.Info("data acquired",
		"username", username,
		"password", password,
	)

	events.Stop()
	return err
}

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	export.SetCommand(exportLogic)
	cli.Execute()
}
```

See the example in action:

![img](examples/textinput/example.gif "TextInput example")


<a id="orga942dec"></a>

### MultiInput widget

The MultiInput widget creates a multi-selection interface that allows users to select multiple items from a predefined list of options:

```go
func MultiInput(ctx context.Context, title string, options []string) ([]string, error)
```

Parameters:

-   **ctx:** Go context for handling Ctrl-C interrupts or timeouts
-   **title:** The selection prompt displayed to the user
-   **options:** The list of selectable items

The following example demonstrates an `exportLogic` function that uses the `MultiInput` widget:

```go
func exportLogic(ctx context.Context, events export.EventHandler) error {
	slog.Info("export command invoked")

	protocols, err := widget.MultiInput(ctx,
		"Select the supported protocols",
		[]string{
			"FTP",
			"HTTP",
			"HTTPS",
			"SFTP",
			"SSH",
		},
	)

	slog.Info("data acquired",
		"protocols", protocols,
	)

	events.Stop()
	return err
}
```

The complete source code is assembled as follows:

```go
package main

import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/widget"
)

func exportLogic(ctx context.Context, events export.EventHandler) error {
	slog.Info("export command invoked")

	protocols, err := widget.MultiInput(ctx,
		"Select the supported protocols",
		[]string{
			"FTP",
			"HTTP",
			"HTTPS",
			"SFTP",
			"SSH",
		},
	)

	slog.Info("data acquired",
		"protocols", protocols,
	)

	events.Stop()
	return err
}

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	export.SetCommand(exportLogic)
	cli.Execute()
}
```

Running this example produces the following output:

![img](examples/multiinput/example.gif "MultiInput example")


<a id="org996445e"></a>

## Configuration parameters

CLI tools built using `xp-clifford` can be configured through several methods:

-   Command-line flags
-   Environment variables
-   Configuration files

`xp-clifford` provides types and functions to facilitate configuration and management of these parameters. Configuration parameter handling is also integrated with the widget capabilities of `xp-clifford`.

Currently, the following configuration parameter types are supported:

-   `bool`
-   `string`
-   `[]string`

All configuration parameters managed by `xp-clifford` implement the `configparam.ConfigParam` interface.


<a id="orgbaaa2af"></a>

### Global configuration parameters

Any CLI tool built using `xp-clifford` includes the following global flags:

-   **`-c` or `--config`:** Configuration file for setting additional parameters (string)
-   **`-v` or `--verbose`:** Enable verbose logging (bool)
-   **`-h` or `--help`:** Print help message (bool)

The verbose logging is explained in [Verbose logging](#verbose). The configuration file handling is elaborated in the [Configuration file](#config-file).


<a id="verbose"></a>

#### Verbose logging

Enable verbose logging with the `-v` or `--verbose` flag. When enabled, structured log messages at the *Debug* level are also printed to the console.

An example `exportLogic` function:

```go
func exportLogic(_ context.Context, events export.EventHandler) error {
	slog.Debug("export command invoked")
	events.Stop()
	return nil
}
```

The complete example:

```go
package main

import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
)

func exportLogic(_ context.Context, events export.EventHandler) error {
	slog.Debug("export command invoked")
	events.Stop()
	return nil
}

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	export.SetCommand(exportLogic)
	cli.Execute()
}
```

Executing the `export` subcommand without the `-v` flag produces no output:

```sh
go run ./examples/verbose/main.go export
```

With the `-v` flag, the debug-level message appears:

```sh
go run ./examples/verbose/main.go export -v
```

    DEBU export command invoked


<a id="org0ebf2aa"></a>

### Configuration parameters of the export subcommand

The `export` subcommand includes the following default configuration parameters:

-   **`-k` or `--kind`:** Resource kinds to export ([]string)
-   **`-o` or `--output`:** Redirect output to a file (string)

You can extend the `export` subcommand with additional configuration parameters using the `export.AddConfigParams` function:

```go
func AddConfigParams(param ...configparam.ConfigParam)
```


<a id="org2139062"></a>

### Bool configuration parameter

Create a new *bool* configuration parameter using the `configparam.Bool` function:

```go
func Bool(name, description string) *BoolParam
```

The two mandatory arguments are *name* and *description*. Fine-tune the parameter with these methods:

-   **`WithShortName`:** Single-character short command-line flag
-   **`WithFlagName`:** Long format of the command-line flag (defaults to *name*)
-   **`WithEnvVarName`:** Environment variable name for the parameter
-   **`WithDefaultValue`:** Default value of the parameter

Use the `Value()` method to retrieve the parameter value. The `IsSet()` method returns true if the user has explicitly set the value.

Here is a bool configuration parameter definition:

```go
var testParam = configparam.Bool("test", "test bool parameter").
        WithShortName("t").
        WithEnvVarName("CLIFFORD_TEST")
```

Add the parameter to the `export` subcommand:

```go
export.AddConfigParams(testParam)
```

A complete working example:

```go
package main

import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
)

func exportLogic(_ context.Context, events export.EventHandler) error {
	slog.Info("export command invoked", "test-value", testParam.Value())
	events.Stop()
	return nil
}

var testParam = configparam.Bool("test", "test bool parameter").
        WithShortName("t").
        WithEnvVarName("CLIFFORD_TEST")

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	export.AddConfigParams(testParam)
	export.SetCommand(exportLogic)
	cli.Execute()
}
```

The new parameter appears in the help output:

```sh
go run ./examples/boolparam/main.go export --help
```

```
Export test system resources and transform them into managed resources that the Crossplane provider can consume

Usage:
  test-exporter export [flags]

Flags:
  -h, --help            help for export
  -k, --kind strings    Resource kinds to export
  -o, --output string   redirect the YAML output to a file
  -t, --test            test bool parameter

Global Flags:
  -c, --config string   Configuration file
  -v, --verbose         Verbose output
```

By default, test is `false`:

```sh
go run ./examples/boolparam/main.go export
```

    INFO export command invoked test-value=false

Enable it using the `--test` flag:

```sh
go run ./examples/boolparam/main.go export --test
```

    INFO export command invoked test-value=true

Or using the shorthand `-t` flag:

```sh
go run ./examples/boolparam/main.go export -t
```

    INFO export command invoked test-value=true

Or using the `CLIFFORD_TEST` environment variable:

```sh
CLIFFORD_TEST=1 go run ./examples/boolparam/main.go export
```

    INFO export command invoked test-value=true


<a id="orgfac3d5b"></a>

### String configuration parameter

Create a new *string* configuration parameter using the `configparam.String` function:

```go
func String(name, description string) *StringParam
```

The two mandatory arguments are *name* and *description*. Fine-tune the parameter with these methods:

-   **`WithShortName`:** Single-character short command-line flag
-   **`WithFlagName`:** Long format of the command-line flag (defaults to *name*)
-   **`WithEnvVarName`:** Environment variable name for the parameter
-   **`WithDefaultValue`:** Default value of the parameter

Use the `Value()` method to retrieve the parameter value. The `IsSet()` method returns true if the user has explicitly set the value.

The `ValueOrAsk` method returns the value if set. Otherwise, it prompts for the value interactively using the `TextInput` widget.

Consider the following string configuration parameter:

```go
var testParam = configparam.String("username", "username used for authentication").
	WithShortName("u").
	WithEnvVarName("USERNAME").
	WithDefaultValue("testuser")
```

A complete example:

```go
package main

import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
)

func exportLogic(ctx context.Context, events export.EventHandler) error {
	slog.Info("export command invoked",
		"username", testParam.Value(),
		"is-set", testParam.IsSet(),
	)

	// If not set, ask the value
	username, err := testParam.ValueOrAsk(ctx)
	if err != nil {
		return err
	}

	slog.Info("value set by user", "value", username)

	events.Stop()
	return nil
}

var testParam = configparam.String("username", "username used for authentication").
	WithShortName("u").
	WithEnvVarName("USERNAME").
	WithDefaultValue("testuser")

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	export.AddConfigParams(testParam)
	export.SetCommand(exportLogic)
        cli.Execute()
}
```

The new parameter appears in the help output:

```sh
go run ./examples/stringparam/main.go export --help
```

```
Export test system resources and transform them into managed resources that the Crossplane provider can consume

Usage:
  test-exporter export [flags]

Flags:
  -h, --help              help for export
  -k, --kind strings      Resource kinds to export
  -o, --output string     redirect the YAML output to a file
  -u, --username string   username used for authentication

Global Flags:
  -c, --config string   Configuration file
  -v, --verbose         Verbose output
```

Set the value using the `--username` flag:

```sh
go run ./examples/stringparam/main.go export --username anonymous
```

    INFO export command invoked username=anonymous is-set=true
    INFO value set by user value=anonymous

Or using the shorthand `-u` flag:

```sh
go run ./examples/stringparam/main.go export -u anonymous
```

    INFO export command invoked username=anonymous is-set=true
    INFO value set by user value=anonymous

Or using the `USERNAME` environment variable:

```sh
USERNAME=anonymous go run ./examples/stringparam/main.go export
```

    INFO export command invoked username=anonymous is-set=true
    INFO value set by user value=anonymous

When no value is provided, the `TextInput` widget prompts for it interactively:

![img](examples/stringparam/example.gif "Asking a string config parameter value")


<a id="orgfd67f35"></a>

### String slice configuration parameter

A string slice configuration parameter configures values of type `[]string`.

Create a new *string slice* configuration parameter using the `configparam.StringSlice` function:

```go
func StringSlice(name, description string) *StringSliceParam
```

The two mandatory arguments are *name* and *description*. Fine-tune the parameter with these methods:

-   **`WithShortName`:** Single-character short command-line flag
-   **`WithFlagName`:** Long format of the command-line flag (defaults to *name*)
-   **`WithEnvVarName`:** Environment variable name for the parameter
-   **`WithDefaultValue`:** Default value of the parameter
-   **`WithPossibleValues`:** Limit the selection options offered during `ValueOrAsk`
-   **`WithPossibleValuesFn`:** Function that provides the selection options offered during `ValueOrAsk`

Use the `Value()` method to retrieve the parameter value. The `IsSet()` method returns true if the user has explicitly set the value.

The `ValueOrAsk` method returns the value if set. Otherwise, it prompts for the value interactively using the `MultiInput` widget. Interactive prompting requires setting possible values with `WithPossibleValues` or `WithPossibleValuesFn`.


<a id="org506ac65"></a>

#### Without possible values

The following example configures a *StringSlice* parameter:

```go
var testParam = configparam.StringSlice("protocol", "list of supported protocols").
	WithShortName("p").
	WithEnvVarName("PROTOCOLS")
```

Complete example:

```go
package main

import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
)

func exportLogic(ctx context.Context, events export.EventHandler) error {
	slog.Info("export command invoked",
		"protocols", testParam.Value(),
		"num-of-protos", len(testParam.Value()),
		"is-set", testParam.IsSet(),
	)

	events.Stop()
	return nil
}

var testParam = configparam.StringSlice("protocol", "list of supported protocols").
	WithShortName("p").
	WithEnvVarName("PROTOCOLS")

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	export.AddConfigParams(testParam)
	export.SetCommand(exportLogic)
        cli.Execute()
}
```

The new parameter appears in the help output:

```sh
go run ./examples/stringslice/main.go export --help
```

```
Export test system resources and transform them into managed resources that the Crossplane provider can consume

Usage:
  test-exporter export [flags]

Flags:
  -h, --help               help for export
  -k, --kind strings       Resource kinds to export
  -o, --output string      redirect the YAML output to a file
  -p, --protocol strings   list of supported protocols

Global Flags:
  -c, --config string   Configuration file
  -v, --verbose         Verbose output
```

Without setting the value:

```sh
go run ./examples/stringslice/main.go export
```

    INFO export command invoked protocols=[] num-of-protos=0 is-set=false

Set the value using the `--protocol` flag:

```sh
go run ./examples/stringslice/main.go export --protocol HTTP --protocol HTTPS --protocol SSH
```

    INFO export command invoked protocols="[HTTP HTTPS SSH]" num-of-protos=3 is-set=true

Set the value using the `-p` flag:

```sh
go run ./examples/stringslice/main.go export -p HTTP -p SFTP -p FTP
```

    INFO export command invoked protocols="[HTTP SFTP FTP]" num-of-protos=3 is-set=true

Set the value using the `PROTOCOLS` environment variable:

```sh
PROTOCOLS="HTTP HTTPS FTP" go run ./examples/stringslice/main.go export
```

    INFO export command invoked protocols="[HTTP HTTPS FTP]" num-of-protos=3 is-set=true


<a id="orgc3863da"></a>

#### With static possible values

To enable interactive prompting with *StringSlice* configuration parameters, add static selection options using the `WithPossibleValues` method.

Define the configuration parameter:

```go
var testParam = configparam.StringSlice("protocol", "list of supported protocols").
	WithShortName("p").
	WithEnvVarName("PROTOCOLS").
	WithPossibleValues([]string{"HTTP", "HTTPS", "FTP", "SSH", "SFTP"})
```

Complete example:

```go
package main

import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
)

func exportLogic(ctx context.Context, events export.EventHandler) error {
	slog.Info("export command invoked",
		"protocols", testParam.Value(),
		"num-of-protos", len(testParam.Value()),
		"is-set", testParam.IsSet(),
	)

	protocols, err := testParam.ValueOrAsk(ctx)
	if err != nil {
		return err
	}

	slog.Info("data acquired", "protocols", protocols)

	events.Stop()
	return nil
}

var testParam = configparam.StringSlice("protocol", "list of supported protocols").
	WithShortName("p").
	WithEnvVarName("PROTOCOLS").
	WithPossibleValues([]string{"HTTP", "HTTPS", "FTP", "SSH", "SFTP"})

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	export.AddConfigParams(testParam)
	export.SetCommand(exportLogic)
        cli.Execute()
}
```

You can set values with flags or environment variables as before:

```sh
go run ./examples/stringslicestatic/main.go export --protocol HTTP --protocol HTTPS --protocol SSH
```

    INFO export command invoked protocols="[HTTP HTTPS SSH]" num-of-protos=3 is-set=true
    INFO data acquired protocols="[HTTP HTTPS SSH]"

```sh
go run ./examples/stringslicestatic/main.go export -p HTTP -p SFTP -p FTP
```

    INFO export command invoked protocols="[HTTP SFTP FTP]" num-of-protos=3 is-set=true
    INFO data acquired protocols="[HTTP SFTP FTP]"

```sh
PROTOCOLS="HTTP HTTPS FTP" go run ./examples/stringslicestatic/main.go export
```

    INFO export command invoked protocols="[HTTP HTTPS FTP]" num-of-protos=3 is-set=true
    INFO data acquired protocols="[HTTP HTTPS FTP]"

When you omit the parameter values, the CLI tool prompts for them interactively:

![img](examples/stringslicestatic/example.gif "Prompting for StringSlice value")


<a id="orgd4baeba"></a>

#### With dynamic possible values

Sometimes the set of possible *StringSlice* parameter values cannot be defined at build time. The value set may depend on a previous interactive selection or the result of an API request.

In such cases, set the possible values dynamically using the `WithPossibleValuesFn` method.

Consider a simple *Bool* configuration parameter:

```go
var secureParam = configparam.Bool("secure", "secure protocol").
        WithShortName("s").
        WithEnvVarName("SECURE")
```

Based on the value of `secureParam`, the `possibleProtocols` function suggests different protocol names:

```go
func possibleProtocols() ([]string, error) {
	if secureParam.Value() {
		return []string{"HTTPS", "SFTP", "SSH"}, nil
	}
	return []string{"FTP", "HTTP"}, nil
}
```

The `protocolsParam` configuration parameter uses `possibleProtocols` when prompting the user with the `ValueOrAsk` method:

```go
var protocolsParam = configparam.StringSlice("protocol", "list of supported protocols").
	WithShortName("p").
	WithEnvVarName("PROTOCOLS").
	WithPossibleValuesFn(possibleProtocols)
```

Complete example:

```go
package main

import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
)

func exportLogic(ctx context.Context, events export.EventHandler) error {
	slog.Info("export command invoked",
	        "secure", secureParam.Value(),
		"secure-is-set", secureParam.IsSet(),
		"protocols", protocolsParam.Value(),
		"num-of-protos", len(protocolsParam.Value()),
		"protocols-is-set", protocolsParam.IsSet(),
	)

	protocols, err := protocolsParam.ValueOrAsk(ctx)
	if err != nil {
		return err
	}

	slog.Info("data acquired", "protocols", protocols)

	events.Stop()
	return nil
}

func possibleProtocols() ([]string, error) {
	if secureParam.Value() {
		return []string{"HTTPS", "SFTP", "SSH"}, nil
	}
	return []string{"FTP", "HTTP"}, nil
}

var secureParam = configparam.Bool("secure", "secure protocol").
        WithShortName("s").
        WithEnvVarName("SECURE")

var protocolsParam = configparam.StringSlice("protocol", "list of supported protocols").
	WithShortName("p").
	WithEnvVarName("PROTOCOLS").
	WithPossibleValuesFn(possibleProtocols)

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	export.AddConfigParams(secureParam, protocolsParam)
	export.SetCommand(exportLogic)
        cli.Execute()
}
```

Both parameters appear in the help output:

```sh
go run ./examples/stringslicedynamic/main.go export --help
```

```
Export test system resources and transform them into managed resources that the Crossplane provider can consume

Usage:
  test-exporter export [flags]

Flags:
  -h, --help               help for export
  -k, --kind strings       Resource kinds to export
  -o, --output string      redirect the YAML output to a file
  -p, --protocol strings   list of supported protocols
  -s, --secure             secure protocol

Global Flags:
  -c, --config string   Configuration file
  -v, --verbose         Verbose output
```

Set the values using flags as usual:

```sh
go run ./examples/stringslicedynamic/main.go export -s --protocol HTTPS --protocol SFTP
```

    INFO export command invoked secure=true secure-is-set=true protocols="[HTTPS SFTP]" num-of-protos=2 protocols-is-set=true
    INFO data acquired protocols="[HTTPS SFTP]"

When the *protocol* configuration parameter is not set, the CLI prompts for its value interactively. The available options depend on the value of *secure*.

If *secure* is not set:

![img](examples/stringslicedynamic/example1.gif "Prompting for StringSlice dynamically - secure is off")

If *secure* is set:

![img](examples/stringslicedynamic/example2.gif "Prompting for StringSlice dynamically - secure is on")


<a id="config-file"></a>

### Configuration file

In addition to CLI flags and environment variables, a CLI tool built with `xp-clifford` can read configuration from a YAML file.

You can specify the configuration file path using the `--config` / `-c` global flag.

If you don't specify a configuration file, the CLI looks for one in these locations, in order:

1.  `$XDG_CONFIG_HOME/<config_file_name>`
2.  `$HOME/<config_file_name>`

The `config_file_name` is `export-cli-config-<shortname>`, where `shortname` is the value of `cli.Configuration.ShortName`.

The YAML file contains key-value pairs, where keys are configuration parameter names in lowercase.

Here is a simple example CLI with three configuration parameters:

```go
package main

import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
)

func exportLogic(ctx context.Context, events export.EventHandler) error {
	slog.Info("export command invoked",
		"protocols", protocolParam.Value(),
		"username", usernameParam.Value(),
		"boolparam", boolParam.Value(),
	)

	events.Stop()
	return nil
}

var protocolParam = configparam.StringSlice("protocol", "list of supported protocols").
	WithShortName("p").
	WithEnvVarName("PROTOCOLS")

var usernameParam = configparam.String("username", "username used for authentication").
	WithShortName("u").
	WithEnvVarName("USERNAME")

var boolParam = configparam.Bool("bool", "test bool parameter").
	WithShortName("b").
	WithEnvVarName("CLIFFORD_BOOL")

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	export.AddConfigParams(protocolParam, usernameParam, boolParam)
	export.SetCommand(exportLogic)
	cli.Execute()
}
```

Flag-based configuration works as expected:

```sh
go run ./examples/configfile/main.go export -b --protocol HTTPS --protocol SFTP --username anonymous
```

    INFO export command invoked protocols="[HTTPS SFTP]" username=anonymous boolparam=true

Without CLI flags:

```sh
go run ./examples/configfile/main.go export
```

    INFO export command invoked protocols=[] username="" boolparam=false

Now let's create a configuration file:

```yaml
protocol:
  - HTTP
  - FTP
username: config-user
bool: true
```

The CLI reads configuration parameter values from this file:

```sh
go run ./examples/configfile/main.go export --config ./examples/configfile/config
```

    INFO export command invoked protocols="[HTTP FTP]" username=config-user boolparam=true

Environment variables override values from the configuration file:

```sh
PROTOCOLS="FTP" go run ./examples/configfile/main.go export --config ./examples/configfile/config
```

    INFO export command invoked protocols=[FTP] username=config-user boolparam=true

CLI flags take the highest precedence and override everything else:

```sh
PROTOCOLS="FTP" go run ./examples/configfile/main.go export --config ./examples/configfile/config --protocol SSH -b=false
```

    INFO export command invoked protocols=[SSH] username=config-user boolparam=false


<a id="orgc358bd5"></a>

## Parsing and sanitizing

When creating Crossplane managed resource definitions, we frequently transform objects describing external resources into a different schema. Usually the values are preserved, but the data structure differs.

Sometimes we cannot preserve values exactly because they must conform to certain rules.

An example is the `metadata.name` field of Kubernetes resources<sup><a id="fnr.1" class="footref" href="#fn.1" role="doc-backlink">1</a></sup>. The Kubernetes documentation references various RFCs and extends those requirements with additional rules.

The `parsan` package in `xp-clifford` provides functions that transform strings into formats satisfying different Kubernetes object name requirements. This process is called sanitization. The `ParseAndSanitize` function performs this action:

```go
func ParseAndSanitize(input string, rule Rule) []string
```

The `ParseAndSanitize` function takes an *input* string and a *rule*, then transforms the *input* to conform to the *rule*. Since multiple valid sanitized solutions may exist, the function returns all of them.


<a id="org50009f6"></a>

### Sanitizer rules

The following rules are available for sanitization.


<a id="org0460922"></a>

#### RFC1035Subdomain

The `RFC1035Subdomain` rule conforms to:

> <subdomain> ::= <label> | <subdomain> "." <label>

A *subdomain* is either a single *label* or multiple *labels* separated by dots (e.g., *label*./label/./label/).

A *label* is a string that:

-   starts with a letter (lowercase or uppercase),
-   ends with a letter (lowercase or uppercase) or a digit,
-   contains only letters, digits, and `-` characters.

A *label* cannot exceed 63 characters. A *subdomain* cannot exceed 253 characters.

During sanitization, invalid characters are replaced with `-` or `x`. The `@` symbol is replaced with `-at-`. Labels and subdomains that are too long are trimmed.

Examples:

| input                | sanitized            |
|-------------------- |-------------------- |
| www.example.com      | www.example.com      |
| Can you sanitize me? | Can-you-sanitize-mex |
| 99Luftballons        | x99Luftballons       |
| admin@example.com    | admin-at-example.com |


<a id="org8cf7fab"></a>

#### RFC1035LowerSubdomain

The `RFC1035LowerSubdomain` rule is a variation of `RFC1035Subdomain` that requires lowercase letters only. Uppercase letters are converted to lowercase:

| input                | sanitized            |
|-------------------- |-------------------- |
| www.example.com      | www.example.com      |
| Can you sanitize me? | can-you-sanitize-mex |
| 99Luftballons        | x99luftballons       |
| admin@example.com    | admin-at-example.com |


<a id="org2ee447c"></a>

#### RFC1035SubdomainRelaxed

The `RFC1035SubdomainRelaxed` rule is a variation of `RFC1035Subdomain` that allows *labels* to start with digits:

| input                | sanitized            |
|-------------------- |-------------------- |
| www.example.com      | www.example.com      |
| Can you sanitize me? | Can-you-sanitize-mex |
| 99Luftballons        | 99Luftballons        |
| admin@example.com    | admin-at-example.com |


<a id="orgd6f79b2"></a>

#### RFC1035LowerSubdomainRelaxed

The `RFC1035LowerSubdomainRelaxed` rule combines `RFC1035LowerSubdomain` and `RFC1035SubdomainRelaxed`. Uppercase characters are converted to lowercase, and *labels* may start with digits:

| input                | sanitized            |
|-------------------- |-------------------- |
| www.example.com      | www.example.com      |
| Can you sanitize me? | can-you-sanitize-mex |
| 99Luftballons        | 99luftballons        |
| admin@example.com    | admin-at-example.com |

## Footnotes

<sup><a id="fn.1" class="footnum" href="#fnr.1">1</a></sup> [Object Names and IDs - kubernetes.io](https://kubernetes.io/docs/concepts/overview/working-with-objects/names/)
