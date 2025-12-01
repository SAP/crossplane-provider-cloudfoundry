- [Introduction](#org0dfa982)
- [Examples](#org73f409c)
  - [The simplest CLI tool](#orgd867774)
  - [Exporting](#orgbc9d9c0)
    - [Basic export subcommand](#org3c00a3a)
    - [Exporting a resource](#org641166f)
    - [Displaying warnings](#orgd14d1a1)
    - [Exporting commented out resources](#org4493378)
  - [Errors with attributes](#erratt-example)
  - [Widgets](#org87b14d6)
    - [TextInput widget](#org5d91800)
  - [MultiInput widget](#org60a5068)



<a id="org0dfa982"></a>

# Introduction

`xp-clifford` (Crossplane CLI Framework for Resource Data Extraction) is a Go module that facilitates the development of CLI tools for exporting definitions of external resources in the format of specific Crossplane provider managed resource definitions.


<a id="org73f409c"></a>

# Examples

These examples demonstrate the basic features of `xp-clifford` and build progressively on one another.


<a id="orgd867774"></a>

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


<a id="orgbc9d9c0"></a>

## Exporting


<a id="org3c00a3a"></a>

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


<a id="org641166f"></a>

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


<a id="orgd14d1a1"></a>

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


<a id="org4493378"></a>

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
    # Object:
    #   password: secret
    #   user: test-user-commented
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


<a id="org87b14d6"></a>

## Widgets

`xp-clifford` provides several CLI widgets to facility the interaction with the user.


<a id="org5d91800"></a>

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


<a id="org60a5068"></a>

## MultiInput widget

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
