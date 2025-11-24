package config

import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/configparam"

	"github.com/cloudfoundry/go-cfclient/v3/config"
)

func Get(ctx context.Context, useCfLoginMethod *configparam.BoolParam, apiUrlParam, usernameParam, passwordParam *configparam.StringParam) (*config.Config, error) {
	if useCfLoginMethod.Value() {
		slog.Debug("log in to CF using the CF login method")
		return config.NewFromCFHome()
	} else {
		apiUrl, err := apiUrlParam.ValueOrAsk(ctx)
		if err != nil {
			return nil, err
		}
		username, err := usernameParam.ValueOrAsk(ctx)
		if err != nil {
			return nil, err
		}
		password, err := passwordParam.ValueOrAsk(ctx)
		if err != nil {
			return nil, err
		}
		slog.Debug("log in to CF using credentials", "url", apiUrl, "username", username)
		return config.New(apiUrl, config.UserPassword(username, password))
	}
}
