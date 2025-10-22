package main

import "github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli"

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	cli.Execute()
}
