package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	subcmd "github.com/SAP/crossplane-provider-cloudfoundry/cmd/importer/cmd/subcmd"
)

var rootCmd = &cobra.Command{
	Use:   "importer [command] [flags...]",
	Short: "Crossplane Cloud Foundry provider importing tool",
	Long:  "Crossplane Cloud Foundry provider importing tool is a CLI tool to import existing Cloud Foundry resources as Crossplane managed resources",
}

// Execute runs the root command and handles errors
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Initialize subcommands
	subcmd.AddInitCMD(rootCmd)
	subcmd.AddImportCMD(rootCmd)
}
