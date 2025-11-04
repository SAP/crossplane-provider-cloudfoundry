package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/config"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/guidname"
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

func getOrgs(ctx context.Context, cfClient *client.Client) (*org.Cache, error) {
	if orgCache != nil {
		return orgCache, nil
	}
	orgsParam.WithPossibleValuesFn(org.GetAllNamesFn(ctx, cfClient))

	selectedOrgs, err := orgsParam.ValueOrAsk(ctx)
	if err != nil {
		return nil, err
	}

	orgNames := make([]string, len(selectedOrgs))
	for i, orgName := range selectedOrgs {
		name, err := guidname.ParseName(orgName)
		if err != nil {
			return nil, err
		}
		orgNames[i] = name.Name
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	orgs, err := org.GetAll(ctx, cfClient, orgNames)
	if err != nil {
		return nil, err
	}
	orgCache = org.New(orgs)
	return orgCache, nil
}

func getSpaces(ctx context.Context, cfClient *client.Client) (*space.Cache, error) {
	if spaceCache != nil {
		return spaceCache, nil
	}
	orgs, err := getOrgs(ctx, cfClient)
	if err != nil {
		return nil, err
	}

	spacesParam.WithPossibleValuesFn(space.GetAllNamesFn(ctx, cfClient, orgs.GetGUIDs()))

	selectedSpaces, err := spacesParam.ValueOrAsk(ctx)
	if err != nil {
		return nil, err
	}

	spaceNames := make([]string, len(selectedSpaces))
	for i, spaceName := range selectedSpaces {
		name, err := guidname.ParseName(spaceName)
		if err != nil {
			return nil, err
		}
		spaceNames[i] = name.Name
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	spaces, err := space.GetAll(ctx, cfClient, orgs.GetGUIDs(), spaceNames)
	if err != nil {
		return nil, erratt.Errorf("cannot get spaces: %w", err)
	}
	spaceCache = space.New(spaces)
	spacesParam.WithPossibleValuesFn(convertPossibleValuesFn(spaceCache.GetNames))
	return spaceCache, nil
}

func getServiceInstances(ctx context.Context, cfClient *client.Client) (*serviceinstance.Cache, error) {
	if serviceInstanceCache != nil {
		return serviceInstanceCache, nil
	}

	orgs, err := getOrgs(ctx, cfClient)
	if err != nil {
		return nil, err
	}

	spaces, err := getSpaces(ctx, cfClient)
	if err != nil {
		return nil, err
	}

	serviceInstanceParam.WithPossibleValuesFn(serviceinstance.GetAllNamesFn(ctx, cfClient, orgs.GetGUIDs(), spaces.GetGUIDs()))

	selectedServiceInstances, err := serviceInstanceParam.ValueOrAsk(ctx)
	if err != nil {
		return nil, err
	}

	serviceInstanceNames := make([]string, len(selectedServiceInstances))
	for i, serviceInstanceName := range selectedServiceInstances {
		name, err := guidname.ParseName(serviceInstanceName)
		if err != nil {
			return nil, err
		}
		serviceInstanceNames[i] = name.Name
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	serviceInstances, err := serviceinstance.GetAll(ctx,
		cfClient,
		orgs.GetGUIDs(),
		spaces.GetGUIDs(),
		serviceInstanceNames,
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
			orgs, err := getOrgs(ctx, cfClient)
			if err != nil {
				return err
			}
			orgs.Export(evHandler)
		case "space":
			spaces, err := getSpaces(ctx, cfClient)
			if err != nil {
				return err
			}
			spaces.Export(evHandler)
		case "serviceinstance":
			serviceInstaces, err := getServiceInstances(ctx, cfClient)
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
	serviceInstanceParam = configparam.StringSlice("serviceinstance", "Filter for Cloud Foundry service instances").
				WithFlagName("serviceinstance")
	useCfLoginMethod = configparam.Bool("use-cf-login", "Reuse the login config generated by 'cf login' command").
				WithEnvVarName("USE_CF_LOGIN")
)

func main() {
	cli.Configuration.ShortName = shortName
	cli.Configuration.ObservedSystem = observedSystem
	export.SetCommand(exportCmd)
	export.AddConfigParams(
		apiUrlParam,
		usernameParam,
		passwordParam,
		orgsParam,
		spacesParam,
		serviceInstanceParam,
		useCfLoginMethod,
	)
	export.AddResourceKinds("organization", "space", "serviceinstance")
	cli.Execute()
}
