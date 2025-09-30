package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"
	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

const (
	shortName      = "cf"
	observedSystem = "Cloud Foundry"
)

var (
	orgCache   *orgDB
	spaceCache *spaceDB
)

func exportCmd(resourceChan chan<- resource.Object, errChan chan<- erratt.ErrorWithAttrs) error {
	fmt.Println(cli.GetExportConfigParams().String())
	cfConfig, err := getCFConfig()
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
	orgCache = newOrgDB(cfClient)
	spaceCache = newSpaceDB(cfClient)
	orgsParam.(*configparam.StringSliceParam).WithPossibleValuesFn(orgCache.getNames)
	spacesParam.(*configparam.StringSliceParam).WithPossibleValuesFn(spaceCache.getNamesByOrgGUIDs)
	selectedResources, err := cli.ResourceKindParam.ValueOrAsk(context.Background())
	if err != nil {
		return erratt.Errorf("cannot get the value for resource kind parameter: %w", err)
	}
	slog.Info("Kinds selected", "resources", selectedResources)
	for _, kind := range selectedResources {
		switch kind {
		case "organization":
			if err := exportOrgs(cfClient, resourceChan); err != nil {
				return err
			}
		case "space":
			if err := exportSpaces(cfClient, resourceChan); err != nil {
				return err
			}
		case "serviceinstance":
			if err := exportServiceInstances(cfClient, resourceChan, errChan); err != nil {
				return err
			}
		default:
			return erratt.New("unknown resource kind specified", "kind", kind)
		}
	}
	return nil
}

var (
	apiUrlParam = configparam.String("API URL", "URL of the Cloud Foundry API").
			WithShortName("a").
			WithFlagName("apiUrl").
			WithEnvVarName("API_URL").
			WithExample("https://api.cf.enterprise.com")
	usernameParam = configparam.String("CF username", "Username at the Cloud Foundry API").
			WithShortName("u").
			WithFlagName("username").
			WithEnvVarName("USERNAME")
	passwordParam = configparam.SensitiveString("CF user password", "Password at the Cloud Foundry API").
			WithShortName("p").
			WithFlagName("password").
			WithEnvVarName("PASSWORD")
	orgsParam = configparam.StringSlice("CF orgs", "Filter for Cloud Foundry organizations").
			WithFlagName("org")
	spacesParam = configparam.StringSlice("CF spaces", "Filter for Cloud Foundry spaces").
			WithFlagName("space")
)

func main() {
	cli.Configuration.ShortName = shortName
	cli.Configuration.ObservedSystem = observedSystem
	cli.SetExportCommand(exportCmd)
	cli.AddExportCommandParams(
		apiUrlParam,
		usernameParam,
		passwordParam,
		orgsParam,
		spacesParam,
	)
	cli.AddExportableResourceKinds("organization", "space", "serviceinstance")
	cli.Execute()
}
