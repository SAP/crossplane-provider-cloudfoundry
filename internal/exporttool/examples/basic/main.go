package main

import (
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli"
	_ "github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/export"
)

func main() {
	cli.Configuration.ShortName = "test"
	cli.Configuration.ObservedSystem = "test system"
	cli.Execute()
}
