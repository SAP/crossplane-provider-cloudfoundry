package kubernetes

import (
	"path/filepath"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var Flags *flag.FlagSet
var overrides *clientcmd.ConfigOverrides

func init() {
	Flags = flag.NewFlagSet("kube", flag.ContinueOnError)
	overrides = &clientcmd.ConfigOverrides{}
	clientcmd.BindOverrideFlags(overrides, Flags, clientcmd.RecommendedConfigOverrideFlags("kube."))
}

func getConfig() (*rest.Config, error) {
	kubeconfig := viper.GetString("kubeconfig")
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		kubeconfigs, err := filepath.Glob(kubeconfig)
		if err != nil {
			return nil, err
		}
		if len(kubeconfigs) > 0 {
			loadingRules.ExplicitPath = kubeconfigs[0]
		}
	}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	return clientConfig.ClientConfig()

}

func NewClient() (client.Client, error) {
	config, err := getConfig()
	if err != nil {
		return nil, err
	}
	scheme, err := getScheme()
	if err != nil {
		return nil, err
	}
	return client.New(config, client.Options{
		Scheme: scheme,
	})
}
