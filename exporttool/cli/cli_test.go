package cli_test

import "github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli"

func ExampleExecute() {
	cli.Configuration.ShortName = "ts"
	cli.Configuration.ObservedSystem = "test system"
	cli.Execute()
}
