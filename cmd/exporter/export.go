package main

import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/config"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/resources"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/erratt"

	"github.com/cloudfoundry/go-cfclient/v3/client"
)

func exportCmd(ctx context.Context, evHandler export.EventHandler) error {
	cfConfig, err := config.Get(ctx, useCfLoginMethod, apiUrlParam, usernameParam, passwordParam)
	if err != nil {
		return err
	}
	cfClient, err := client.New(cfConfig)
	if err != nil {
		return err
	}
	slog.Debug("connected to CF API", "apiURL", cfConfig.ApiURL("/"))

	selectedResources, err := export.ResourceKindParam.ValueOrAsk(ctx)
	if err != nil {
		return erratt.Errorf("cannot get the value for resource kind parameter: %w", err)
	}
	slog.Debug("kinds selected", "kinds", selectedResources)
	for _, kind := range selectedResources {
		if eFn := resources.ExportFn(kind); eFn != nil {
			if err := eFn(ctx, cfClient, evHandler, resolveRefencesParam.Value()); err != nil {
				return err
			}
		} else {
			return erratt.New("unknown resource kind specified", "kind", kind)
		}
	}
	evHandler.Stop()
	return nil
}
