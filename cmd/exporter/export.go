package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/config"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/org"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/serviceinstance"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/space"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/export"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"

	"github.com/cloudfoundry/go-cfclient/v3/client"
)

//nolint:gocyclo
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
	slog.Debug("Kinds selected", "resources", selectedResources)
	for _, kind := range selectedResources {
		switch kind {
		case "organization":
			orgs, err := org.Get(ctx, cfClient)
			if err != nil {
				return err
			}
			orgs.Export(evHandler)
		case "space":
			spaces, err := space.Get(ctx, cfClient)
			if err != nil {
				return err
			}
			spaces.Export(evHandler)
		case "serviceinstance":
			serviceInstaces, err := serviceinstance.Get(ctx, cfClient)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
			defer cancel()
			serviceInstaces.Export(ctx, cfClient, evHandler)
		default:
			return erratt.New("unknown resource kind specified", "kind", kind)
		}
	}
	evHandler.Stop()
	return nil
}
