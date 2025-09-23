package main

import (
	"fmt"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"
)

const (
	shortName      = "cf"
	observedSystem = "Cloud Foundry"
)

func exportCmd() error {
	fmt.Println(cli.GetExportConfigParams().String())
	_, err := getCFConfig()
	if err != nil {
		return err
	}
	slog.Info("Connected to Cloud Foundry API",
		"URL", apiUrlParam.ValueAsString(),
		"user", usernameParam.ValueAsString(),
	)
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
)

func main() {
	cli.Configuration.ShortName = shortName
	cli.Configuration.ObservedSystem = observedSystem
	cli.SetExportCommand(exportCmd)
	cli.AddExportCommandParams(apiUrlParam, usernameParam, passwordParam)
	cli.Execute()
}
