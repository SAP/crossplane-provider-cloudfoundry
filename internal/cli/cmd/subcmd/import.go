package subcmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	v1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/adapters"
	adapterv1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/adapters/v1alpha1"
	cli "github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/pkg/credentialManager"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/pkg/utils"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/importer"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/resource"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gopkg.in/alecthomas/kingpin.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var preview bool

var (
	// error messages
	errCreateReader = "Could not create a reader for the console prompt"
	errAddv1beta1Scheme = "Could not add v1beta1 scheme"
	errAddv1alpha1Scheme = "Could not add v1alpha1 scheme"
	errAddCorev1Scheme = "Could not add corev1 scheme"
	errImportResources = "Could not import resources"
	errCreateResource = "Could not create resources"
)

// Define colors for this file
var suggestionColorLocal = color.New(color.FgCyan).SprintFunc()

var ImportCMD = &cobra.Command{
	Use:   "import",
	Short: "Import Cloud Foundry resources",
	Long:  `Import the Cloud Foundry resources you defined in your config.yaml. Make sure to first run xpcfi init first.`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.UpdateTransactionID()
		fmt.Println(strings.Repeat("-", 52))
		fmt.Println("| Import Run: " + cli.RetrieveTransactionID() + " |")
		fmt.Println(strings.Repeat("-", 52))

		ctx := context.TODO()
		kubeConfigPath := cli.RetrieveKubeConfigPath()

		// Create schemes
		scheme := runtime.NewScheme()
		err := v1beta1.SchemeBuilder.AddToScheme(scheme)
		kingpin.FatalIfError(err, "%s", errAddv1beta1Scheme)
		err = corev1.SchemeBuilder.AddToScheme(scheme)
		kingpin.FatalIfError(err, "%s", errAddCorev1Scheme)
		err = v1alpha1.SchemeBuilder.AddToScheme(scheme)
		kingpin.FatalIfError(err, "%s", errAddv1alpha1Scheme)

		// Create adapters
		clientAdapter := &adapters.CFClientAdapter{}
		resourceAdapters := map[string]resource.ResourceAdapter{
			v1alpha1.Space_Kind: &adapterv1alpha1.CFSpaceAdapter{},
			v1alpha1.Org_Kind: &adapterv1alpha1.CFOrganizationAdapter{},
			v1alpha1.App_Kind: &adapterv1alpha1.CFAppAdapter{},
		}
		configParser := &adapters.CFConfigParser{}

		// Create importer
		importer := &importer.Importer{
			ClientAdapter:    clientAdapter,
			ResourceAdapters: resourceAdapters,
			ConfigParser:     configParser,
			Scheme:           scheme,
		}

		// Import resources
		if configPath == "" {
			configPath = cli.RetrieveConfigPath()
		}
		
		resources, err := importer.ImportResources(ctx, configPath, kubeConfigPath, importer.Scheme)
		kingpin.FatalIfError(err, "%s", errImportResources)

		if len(resources) == 0 {
			fmt.Println("🛑 Stopped importing, no resources found to import")
			return
		}

		// optional preview of resources
		if preview {
			importer.PreviewResources(resources)
		}

		if !boolPrompt("Do you want to create these resources in your ManagedControlPlane (MCP)? [YES|NO]") {
			fmt.Println("🛑 Stopped importing, no changes were made to your ManagedControlPlane (MCP)")
			return
		}

		// Create resources
		transactionID := cli.RetrieveTransactionID()
		err = importer.CreateResources(ctx, resources, kubeConfigPath, transactionID)
		kingpin.FatalIfError(err, "%s", errCreateResource)

		fmt.Println("✅ Resource(s) successfully imported")
		fmt.Println("\n If you want to revert the import run:")
		fmt.Println(suggestionColorLocal("kubectl delete <RESOURCE TYPE> -l import-ID=" + transactionID))
	},
}

func AddImportCMD(rootCmd *cobra.Command) {
	rootCmd.AddCommand(ImportCMD)
	ImportCMD.Flags().BoolVarP(&preview, "preview", "p", false, "Get a detailed overview on importable resources")
	ImportCMD.Flags().StringVarP(&configPath, "config", "c", "", "Path to your Import-Config (default ./config.yaml)")
}

func boolPrompt(question string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(question)

	input, err := reader.ReadString('\n')
	if err != nil {
		kingpin.FatalIfError(err, "%s", errCreateReader)
	}

	input = strings.TrimSpace(strings.ToUpper(input))
	switch input {
	case "Y", "YES":
		return true
	default:
		return false
	}
}
