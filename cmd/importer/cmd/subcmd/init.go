package subcmd

import (
	"context"
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gopkg.in/alecthomas/kingpin.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	v1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1alpha1"
	v1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/adapters"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/pkg/credentialManager"
)

var (
	successColor    = color.New(color.FgGreen).SprintFunc()
	suggestionColor = color.New(color.FgCyan).SprintFunc()
	kubeConfigPath  string
	configPath      string
)

var (
	errParseConfig = "Could not parse config file"
)

var InitCMD = &cobra.Command{
	Use:   "init",
	Short: "Initializes environment",
	Long:  `Creates an env-file for storing authentication details`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.TODO()

		// Create schemes
		scheme := runtime.NewScheme()
		err := v1beta1.SchemeBuilder.AddToScheme(scheme)
		kingpin.FatalIfError(err, "%s", errAddv1beta1Scheme)
		err = v1alpha1.SchemeBuilder.AddToScheme(scheme)
		kingpin.FatalIfError(err, "%s", errAddv1alpha1Scheme)
		err = corev1.AddToScheme(scheme)
		kingpin.FatalIfError(err, "%s", errAddCorev1Scheme)

		// Create adapters
		clientAdapter := &adapters.CFClientAdapter{}
		configParser := &adapters.CFConfigParser{}

		// if no config path is provided, fallback to default
		if configPath == "" {
			configPath = "./config.yaml"
		}

		providerConfigRef, _, err := configParser.ParseConfig(configPath)
		kingpin.FatalIfError(err, "%s", errParseConfig)

		credentialManager.CreateEnvironment(kubeConfigPath, configPath, ctx, providerConfigRef.GetProviderConfigRef().Namespace, providerConfigRef.GetProviderConfigRef().Name, *clientAdapter, scheme)

		fmt.Println(successColor("\nReady..."))
		fmt.Println("\nStart your import with:")
		fmt.Println(suggestionColor("importer import [--preview | -p]"))
	},
}

// AddInitCMD adds the init command to the root command
func AddInitCMD(rootCmd *cobra.Command) {
	rootCmd.AddCommand(InitCMD)
	InitCMD.Flags().StringVarP(&kubeConfigPath, "kube", "k", "", "Path to your kubeConfig (default ~/.kube/config)")
	InitCMD.Flags().StringVarP(&configPath, "config", "c", "", "Path to your config (default ./config.yaml)")
}
