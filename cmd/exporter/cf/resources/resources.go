package resources

import (
	"context"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/export"

	"github.com/cloudfoundry/go-cfclient/v3/client"
)

// Kind interface must be implemented by each CF resource kinds.
type Kind interface {
	// Param method returns the configuration parameters specific
	// to a resource kind.
	Param() configparam.ConfigParam
	// Export method performs the export operation of a resource
	// kind. The method first identifies the resources that are to
	// be exportd using the values of the related configuration
	// parameters. Then it collects the resource definitions
	// through the cfClient. Finally, the resources are exported
	// using the eventHandler.
	Export(ctx context.Context, cfClient *client.Client, evHandler export.EventHandler) error
}

var kinds = map[string]Kind{}

// RegisterKind function registers a resource kind.
func RegisterKind(kind Kind) {
	kinds[kind.Param().GetName()] = kind
}

// ConfigParams() function returns the configuration parameters of all
// registered resource kinds.
func ConfigParams() []configparam.ConfigParam {
	result := make([]configparam.ConfigParam, len(kinds))
	i := 0
	for _, kind := range kinds {
		result[i] = kind.Param()
		i++
	}
	return result
}

// ExportFn returns the export function of a given kind.
func ExportFn(kind string) func(context.Context, *client.Client, export.EventHandler) error {
	resource, ok := kinds[kind]
	if !ok || resource == nil {
		return nil
	}
	return resource.Export
}
