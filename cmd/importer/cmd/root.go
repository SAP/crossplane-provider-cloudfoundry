package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	subcmd "github.com/SAP/crossplane-provider-cloudfoundry/cmd/importer/cmd/subcmd"
)

var rootCmd = &cobra.Command{
	Use:   "xpcfi",
	Short: "Crossplane-Cloud-Foundry-Importing (XPCFI)",
	Long:  "XPCFI (Crossplane-Cloud-Foundry-Importing) is a CLI tool to import pre-existing Cloud Foundry resources into your ManagedControlPlane (MCP)",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Welcome to XPCFI! Use --help for more information.")
	},
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
