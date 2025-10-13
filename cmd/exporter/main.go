package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/config"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/org"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/serviceinstance"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/space"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/export"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"

	"github.com/cloudfoundry/go-cfclient/v3/client"
)

const (
	shortName      = "cf"
	observedSystem = "Cloud Foundry"
)

var (
	orgCache             *org.Cache
	spaceCache           *space.Cache
	serviceInstanceCache *serviceinstance.Cache
)

func getOrgs(cfClient *client.Client) (*org.Cache, error) {
	if orgCache != nil {
		return orgCache, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	orgs, err := org.GetAll(ctx, cfClient)
	if err != nil {
		return nil, erratt.Errorf("cannot get organizations: %w", err)
	}
	orgCache = org.New(orgs)
	orgsParam.WithPossibleValuesFn(convertPossibleValuesFn(orgCache.GetNames))
	return orgCache, nil
}

func getSpaces(cfClient *client.Client) (*space.Cache, error) {
	if spaceCache != nil {
		return spaceCache, nil
	}
	orgs, err := getOrgs(cfClient)
	if err != nil {
		return nil, err
	}
	selectedOrgs, err := orgsParam.ValueOrAsk()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	spaces, err := space.GetAll(ctx, cfClient, orgs.GetGuidsByNames(selectedOrgs))
	if err != nil {
		return nil, erratt.Errorf("cannot get spaces: %w", err)
	}
	spaceCache = space.New(spaces)
	spacesParam.WithPossibleValuesFn(convertPossibleValuesFn(spaceCache.GetNames))
	return spaceCache, nil
}

func getServiceInstances(cfClient *client.Client) (*serviceinstance.Cache, error) {
	if serviceInstanceCache != nil {
		return serviceInstanceCache, nil
	}
	orgs, err := getOrgs(cfClient)
	if err != nil {
		return nil, err
	}
	selectedOrgs, err := orgsParam.ValueOrAsk()
	if err != nil {
		return nil, err
	}

	spaces, err := getSpaces(cfClient)
	if err != nil {
		return nil, err
	}
	selectedSpaces, err := spacesParam.ValueOrAsk()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	serviceInstances, err := serviceinstance.GetAll(ctx,
		cfClient,
		orgs.GetGuidsByNames(selectedOrgs),
		spaces.GetGuidsByNames(selectedSpaces),
	)
	if err != nil {
		return nil, err
	}
	serviceInstanceCache = serviceinstance.New(serviceInstances)
	return serviceInstanceCache, nil
}

func convertPossibleValuesFn(fn func() []string) func() ([]string, error) {
	return func() ([]string, error) {
		return fn(), nil
	}
}

//nolint:gocyclo
func exportCmd(evHandler export.EventHandler) error {
	cfConfig, err := config.Get(apiUrlParam, usernameParam, passwordParam)
	if err != nil {
		return err
	}
	cfClient, err := client.New(cfConfig)
	if err != nil {
		return err
	}

	slog.Info("Connected to Cloud Foundry API",
		"URL", apiUrlParam.ValueAsString(),
		"user", usernameParam.ValueAsString(),
	)

	selectedResources, err := export.ResourceKindParam.ValueOrAsk()
	if err != nil {
		return erratt.Errorf("cannot get the value for resource kind parameter: %w", err)
	}
	slog.Info("Kinds selected", "resources", selectedResources)
	for _, kind := range selectedResources {
		switch kind {
		case "organization":
			orgs, err := getOrgs(cfClient)
			if err != nil {
				return err
			}
			orgs.Export(evHandler)
		case "space":
			spaces, err := getSpaces(cfClient)
			if err != nil {
				return err
			}
			spaces.Export(evHandler)
		case "serviceinstance":
			serviceInstaces, err := getServiceInstances(cfClient)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			serviceInstaces.Export(ctx, cfClient, evHandler)
		default:
			return erratt.New("unknown resource kind specified", "kind", kind)
		}
	}
	return nil
}

var (
	apiUrlParam = configparam.String("apiUrl", "URL of the Cloud Foundry API").
			WithShortName("a").
			WithFlagName("apiUrl").
			WithEnvVarName("API_URL").
			WithExample("https://api.cf.enterprise.com")
	usernameParam = configparam.String("username", "Username at the Cloud Foundry API").
			WithShortName("u").
			WithFlagName("username").
			WithEnvVarName("USERNAME")
	passwordParam = configparam.SensitiveString("password", "Password at the Cloud Foundry API").
			WithShortName("p").
			WithFlagName("password").
			WithEnvVarName("PASSWORD")
	orgsParam = configparam.StringSlice("org", "Filter for Cloud Foundry organizations").
			WithFlagName("org")
	spacesParam = configparam.StringSlice("space", "Filter for Cloud Foundry spaces").
			WithFlagName("space")
)

func main() {
	cli.Configuration.ShortName = shortName
	cli.Configuration.ObservedSystem = observedSystem
	export.SetCommand(exportCmd)
	export.AddCommandParams(
		apiUrlParam,
		usernameParam,
		passwordParam,
		orgsParam,
		spacesParam,
	)
	export.AddResourceKinds("organization", "space", "serviceinstance")
	cli.Execute()
}
