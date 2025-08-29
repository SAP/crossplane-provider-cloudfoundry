package importcmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/log"
	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	yaml "go.yaml.in/yaml/v3"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	adapterv1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/adapters/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/erratt"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/importer"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/kubernetes"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/provider"
)

func init() {
	ImportCMD.Flags().BoolP("verbose", "v", false, "Verbose output")
	ImportCMD.Flags().BoolP("preview", "p", false, "Get a detailed overview on importable resources")
	ImportCMD.Flags().StringP("config", "c", "config", "Get a detailed overview on importable resources")
	ImportCMD.Flags().String("providerconfig.name", "default", "Name of the ProviderConfig resource")
	ImportCMD.Flags().String("kubeconfig", "$HOME/.kube/config", "Name of the ProviderConfig resource")
	ImportCMD.Flags().AddFlagSet(kubernetes.Flags)
	err := viper.BindEnv("kubeconfig", "KUBECONFIG")
	if err != nil {
		panic(err)
	}
	err = viper.BindPFlags(ImportCMD.Flags())
	if err != nil {
		panic(err)
	}
}

// Define colors for this file
var suggestionColorLocal = color.New(color.FgCyan).SprintFunc()

func setLogger() {
	level := log.InfoLevel
	if viper.GetBool("verbose") {
		level = log.DebugLevel
	}
	slog.SetDefault(slog.New(log.NewWithOptions(os.Stdout, log.Options{
		Level: level,
	})))
}

var ImportCMD = &cobra.Command{
	Use:   "import",
	Short: "Import Cloud Foundry resources",
	Long:  `Import the Cloud Foundry resources you defined in your config.yaml. Make sure to first run importer init first.`,
	Run: func(cmd *cobra.Command, args []string) {
		setLogger()
		slog.Info("Cloud Foundry Import started")
		viper.AddConfigPath(".")
		viper.SetConfigName(viper.GetString("config"))
		viper.SetConfigType("yaml")
		if err := viper.ReadInConfig(); err != nil {
			if errors.As(err, &viper.ConfigFileNotFoundError{}) {
				// if _, ok := err.(viper.ConfigFileNotFoundError); ok {
				slog.Warn("Config file not found", "config", viper.GetString("config"))
			} else {
				panic(err)
			}
		} else {
			slog.Debug("config file read", "config", viper.GetString("config"))
		}
		if viper.GetBool("verbose") {
			in := "Import tool configuration\n```yaml\n" +
				yamlStringSettings() +
				"```\n"
			out, err := glamour.Render(in, "auto")
			if err != nil {
				slog.Error("cannot render configuration as YAML", "err", err)
				return
			}
			fmt.Println(out)
		}

		ctx := context.Background()
		resourceAdapters := map[string]provider.ResourceAdapter{
			v1alpha1.Space_Kind:           &adapterv1alpha1.CFSpaceAdapter{},
			v1alpha1.Org_Kind:             &adapterv1alpha1.CFOrganizationAdapter{},
			v1alpha1.App_Kind:             &adapterv1alpha1.CFAppAdapter{},
			v1alpha1.ServiceInstance_Kind: &adapterv1alpha1.CFServiceInstanceAdapter{},
			v1alpha1.SpaceMembersKind:     &adapterv1alpha1.CFSpaceMembersAdapter{},
			v1alpha1.OrgMembersKind:       &adapterv1alpha1.CFOrgMembersAdapter{},
		}

		// Create importer
		importer := &importer.Importer{
			ResourceAdapters: resourceAdapters,
		}

		kubeClient, err := kubernetes.NewClient()
		if err != nil {
			slog.Error("cannot create kubernetes client", "err", err)
		}

		resources, err := importer.ImportResources(ctx, kubeClient)
		if err != nil {
			erratt.SLog("Cannot import resources", err)
		}

		if len(resources) == 0 {
			slog.Warn("🛑 Stopped importing, no resources found to import")
			return
		}

		// optional preview of resources
		if viper.GetBool("preview") {
			importer.PreviewResources(resources)
		}

		if !boolPrompt("Do you want to create these resources in your Kubernetes cluster? [YES|NO]") {
			slog.Warn("🛑 Stopped importing, no changes were made to your Kubernetes cluster")
			return
		}

		transactionID := uuid.New().String()
		err = importer.CreateResources(ctx, kubeClient, resources, transactionID)
		if err != nil {
			erratt.SLog("Error creating resources", err)
		}

		slog.Info("✅ Resource(s) successfully imported")
		slog.Info("\n If you want to revert the import run:")
		slog.Info(suggestionColorLocal("kubectl delete <RESOURCE TYPE> -l import-ID=" + transactionID))
	},
}

func yamlStringSettings() string {
	c := viper.AllSettings()
	bs, err := yaml.Marshal(c)
	if err != nil {
		slog.Error("marshalling configuration", "error", err)
		os.Exit(1)
	}
	return string(bs)
}

func AddImportCMD(rootCmd *cobra.Command) {
	rootCmd.AddCommand(ImportCMD)
}

func boolPrompt(question string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(question)

	input, err := reader.ReadString('\n')
	if err != nil {
		slog.Error("error creating string reader", "error", err)
		os.Exit(1)
	}

	input = strings.TrimSpace(strings.ToUpper(input))
	switch input {
	case "Y", "YES":
		return true
	default:
		return false
	}
}
