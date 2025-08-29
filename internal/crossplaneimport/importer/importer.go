package importer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/spf13/viper"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/adapters"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/pkg/credentialManager"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/erratt"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/provider"
)

// Importer is the main struct for importing resources
type Importer struct {
	ResourceAdapters map[string]provider.ResourceAdapter
}

func (i *Importer) importOrgs() ([]provider.ResourceFilter, error) {
	orgs := adapters.OrgConfigs{}
	err := viper.UnmarshalKey("resources.orgs", &orgs)
	if err != nil {
		return nil, erratt.Wrap("Unmarshalling orgs in config", err)
	}
	return orgs.ToResourceFilter(), nil
}

func (i *Importer) importSpaces() ([]provider.ResourceFilter, error) {
	spaces := adapters.SpaceConfigs{}
	err := viper.UnmarshalKey("resources.spaces", &spaces)
	if err != nil {
		return nil, erratt.Wrap("Unmarshalling spaces in config", err)
	}
	return spaces.ToResourceFilter(), nil
}

func (i *Importer) importApps() ([]provider.ResourceFilter, error) {
	apps := adapters.AppConfigs{}
	err := viper.UnmarshalKey("resources.apps", &apps)
	if err != nil {
		return nil, erratt.Wrap("Unmarshalling apps in config", err)
	}
	return apps.ToResourceFilter(), nil
}

type resourceFilterCollector func() ([]provider.ResourceFilter, error)

func collectResourceFilters(collectors ...resourceFilterCollector) ([]provider.ResourceFilter, error) {
	result := []provider.ResourceFilter{}
	for _, collector := range collectors {
		filters, err := collector()
		if err != nil {
			return nil, err
		}
		result = append(result, filters...)
	}
	return result, nil
}

// ImportResources imports resources using the provided adapters
func (i *Importer) ImportResources(ctx context.Context,
	kubeClient client.Client) ([]provider.Resource, error) {
	creds, err := credentialManager.RetrieveCredentials(ctx, kubeClient)
	if err != nil {
		return nil, err
	}

	resourceFilters, err := collectResourceFilters(
		i.importOrgs,
		i.importSpaces,
		i.importApps,
	)
	if err != nil {
		return nil, erratt.Wrap("cannot import resources", err)
	}
	var allResources []provider.Resource
	for _, filter := range resourceFilters {
		resourceType := filter.GetResourceType()
		adapter, exists := i.ResourceAdapters[resourceType]
		if !exists {
			return nil, erratt.S("no adapter for for resource type",
				slog.String("resource-type", resourceType),
			)
		}

		// Build client for this adapter
		slog.Debug("connecting to external system")
		if err := adapter.Connect(ctx, creds); err != nil {
			return nil, erratt.S("failed to connect to provider for resource type",
				slog.String("resource-type", resourceType),
				slog.Any("error", err),
			)

		}

		resources, err := adapter.FetchResources(ctx, filter)
		if err != nil {
			return nil, erratt.S("failed to fetch resource",
				slog.String("resource-type", resourceType),
				slog.Any("error", err),
			)
		}

		// Set provider config reference and management policies
		for _, res := range resources {
			res.SetProviderConfigReference(&v1.Reference{
				Name: viper.GetString("providerconfig.name"),
			})
		}

		allResources = append(allResources, resources...)
	}

	return allResources, nil
}

// PreviewResources previews the resources to be imported
func (i *Importer) PreviewResources(resources []provider.Resource) {
	fmt.Println(strings.Repeat("-", 80))
	for _, res := range resources {
		adapter, exists := i.ResourceAdapters[res.GetResourceType()]
		if exists {
			adapter.PreviewResource(res)
		}
	}
}

// CreateResources creates the imported resources in Kubernetes
func (i *Importer) CreateResources(ctx context.Context, k8sClient client.Client, resources []provider.Resource, transactionID string) error {
	// Create resources
	for _, res := range resources {
		managedRes := res.GetManagedResource()

		// Add transaction ID annotation
		annotations := managedRes.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations["import-ID"] = transactionID
		managedRes.SetAnnotations(annotations)

		// Create the resource
		err := k8sClient.Create(ctx, managedRes)
		if err != nil {
			return erratt.S("failed to crate resource",
				slog.String("external-name", res.GetExternalID()),
				slog.Any("error", err))
		}
	}

	return nil
}
