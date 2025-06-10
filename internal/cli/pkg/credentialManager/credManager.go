package credentialManager

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/adapters"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/pkg/utils"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/client"
)

const ConfigName = ".xpcfi"

const (
	// error messages
	errCreateRESTConfig         = "REST config could not be created"
	errAddProviderScheme        = "Unable to add ProviderConfig scheme"
	errAddSecretScheme          = "Unable to add Secret scheme"
	errCreateK8sClient          = "Error creating Kubernetes client"
	errGetProviderConfig        = "Failed to get ProviderConfig"
	errGetSecret                = "Failed to get Secret"
	errExtractCredentials       = "Credentials key not found in secret data"
	errUnmarshalCredentials     = "Failed to unmarshal credentials JSON"
	errCloseCredEnvironmentfile = "Could not close CredEnvironment (file)"
	errOpenCredEnvironment      = "Could not open CredEnvironment"
	errParseCredEnvironment     = "Could not parse CredEnvironment"
	errStoreKubeConfigPath      = "Could not store KubeConfigPath"
	errGetCredentials           = "Could not get credentials"
)

const (
	// CredEnvironment attribute names
	fieldTransactionID  = "TRANSACTION_ID"
	fieldKubeConfigPath = "KUBECONFIGPATH"
	fieldConfigPath     = "CONFIGPATH"
	fieldCredentials    = "CREDENTIALS"
)

func CreateEnvironment(kubeConfigPath string, configPath string, ctx context.Context, providerConfigNamespace string, providerConfigName string, clientAdapter adapters.CFClientAdapter, scheme *runtime.Scheme) {
	// If no kubeConfigPath is provided fall back to ~/.kube/config
	if kubeConfigPath == "" {
		kubeConfigPath = getKubeConfigFallBackPath()
	}
	creds, err := clientAdapter.GetCredentials(ctx, kubeConfigPath, client.ProviderConfigRef{Name: providerConfigName, Namespace: providerConfigNamespace}, scheme)

	kingpin.FatalIfError(err, errGetCredentials)

	storeCredentials(creds, kubeConfigPath, configPath)
}

func storeCredentials(creds client.Credentials, kubeConfigPath string, configPath string) {
	jsonCreds, err := json.Marshal(creds)
	kingpin.FatalIfError(err, errUnmarshalCredentials)
	env := map[string]string{
		fieldTransactionID:  "pending",
		fieldKubeConfigPath: kubeConfigPath,
		fieldConfigPath:     configPath,
		fieldCredentials:    string(jsonCreds),
	}

	utils.StoreKeyValues(env)
	fmt.Println("Import Environment created ...")
}

func getKubeConfigFallBackPath() string {
	// Get the home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Define the .kube/config path
	kubeConfigPath := filepath.Join(homeDir, ".kube", "config")

	// Get the absolute path
	absPath, err := filepath.Abs(kubeConfigPath)
	if err != nil {
		return ""
	}

	return absPath
}

func RetrieveCredentials() client.Credentials {
	file := utils.OpenFile(ConfigName)
	defer func() {
		err := file.Close()
		kingpin.FatalIfError(err, "error closing config file")
	}()
	config := make(map[string]string)

	// Read the file line by line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Split the line by "="
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := parts[0]
			value := parts[1]
			// Store key-value pairs in the map
			config[key] = value
		}
	}

	err := scanner.Err()
	kingpin.FatalIfError(err, errParseCredEnvironment)

	// map values and resturn Credentials
	var credentials adapters.CFCredentials
	err = json.Unmarshal([]byte(config[fieldCredentials]), &credentials)
	kingpin.FatalIfError(err, errUnmarshalCredentials)
	return &credentials
}

func RetrieveKubeConfigPath() string {
	file := utils.OpenFile(ConfigName)
	defer func() {
		err := file.Close()
		kingpin.FatalIfError(err, "error closing config file")
	}()

	// Read the file line by line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Split the line by "="
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && parts[0] == fieldKubeConfigPath {
			return parts[1]
		}
	}
	return getKubeConfigFallBackPath()
}

func RetrieveTransactionID() string {
	file := utils.OpenFile(ConfigName)
	defer func() {
		err := file.Close()
		kingpin.FatalIfError(err, "error closing config file")
	}()

	// Read the file line by line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Split the line by "="
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && parts[0] == fieldTransactionID {
			return parts[1]
		}
	}
	return ""
}

func RetrieveConfigPath() string {
	file := utils.OpenFile(ConfigName)
	defer func() {
		err := file.Close()
		kingpin.FatalIfError(err, "error closing config file")
	}()

	// Read the file line by line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Split the line by "="
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && parts[0] == fieldConfigPath {
			return parts[1]
		}
	}
	return ""
}
