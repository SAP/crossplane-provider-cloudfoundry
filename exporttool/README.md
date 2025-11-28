- [Introduction](#org8564add)
- [Examples](#org2786fe5)
  - [The simplest CLI tool](#org652fca0)
  - [Basic export subcommand](#org95f926c)
  - [Exporting a Resource](#orgd9241cd)



<a id="org8564add"></a>

# Introduction

`xp-clifford` (Crossplane CLI Framework for Resource Data Extraction) is a Go module that facilitates the development of CLI tools for exporting definitions of external resources in the format of specific Crossplane provider managed resource definitions.


<a id="org2786fe5"></a>

# Examples

These examples demonstrate the basic features of `xp-clifford` and build progressively on one another.


<a id="org652fca0"></a>

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

The `main` function:

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


<a id="org95f926c"></a>

## Basic export subcommand

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


<a id="orgd9241cd"></a>

## Exporting a Resource

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
