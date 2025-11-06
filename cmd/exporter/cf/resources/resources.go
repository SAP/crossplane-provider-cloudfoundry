package resources

import (
	"context"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/export"

	"github.com/cloudfoundry/go-cfclient/v3/client"
)

type Kind interface {
	Param() configparam.ConfigParam
	Export(context.Context, *client.Client, export.EventHandler) error
}

var Kinds = map[string]Kind{}

func RegisterKind(name string, kind Kind) {
	Kinds[name] = kind
}

func ConfigParams() []configparam.ConfigParam {
	result := make([]configparam.ConfigParam, len(Kinds))
	i := 0
	for _, kind := range Kinds {
		result[i] = kind.Param()
		i++
	}
	return result
}

func ExportFn(kind string) func(context.Context, *client.Client, export.EventHandler) error {
	resource, ok := Kinds[kind]
	if !ok || resource == nil {
		return nil
	}
	return resource.Export
}
