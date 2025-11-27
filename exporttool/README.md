- [Introduction](#org4afe092)
- [Examples](#orgc7c8de2)
  - [The simplest CLI tool](#orgc97ee90)



<a id="org4afe092"></a>

# Introduction

`xp-clifford` (Crossplane CLI Framework for Resource Data Extraction) is a Go module that facilitates the development of CLI tools for exporting definitions of external resources in the format of specific Crossplane provider managed resource definitions.


<a id="orgc7c8de2"></a>

# Examples


<a id="orgc97ee90"></a>

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
