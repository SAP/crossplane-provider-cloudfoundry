package cli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"
	"github.com/spf13/viper"
)

func init() {
	RegisterConfigModule(configureConfigFile)
}

func defaultConfigFileName() string {
	return fmt.Sprintf("config-%s", Configuration.CLIConfiguration.ShortName)
}

var ConfigFileParam = configparam.String("config", "Configuration file").WithShortName("c")

func configureConfigFile() error {
	configFilePath := "."
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		configFilePath = filepath.Join(v, defaultConfigFileName())
	} else if v := os.Getenv("HOME"); v != "" {
		configFilePath = filepath.Join(v, defaultConfigFileName())
	}
	ConfigFileParam.WithDefaultValue(configFilePath)
	viper.SetConfigFile(configFilePath)
	ConfigFileParam.AttachToCommand(Command)
	// viper.SetConfigName(defaultConfigFileName())
	viper.SetConfigType("yaml")
	// if ConfigFileParam.IsSet() {
	// 	viper.AddConfigPath(ConfigFileParam.(*configparam.StringParam).Value())
	// }
	// if home := os.Getenv("XDG_CONFIG_HOME"); home != "" {
	// 	viper.AddConfigPath("$XDG_CONFIG_HOME")
	// }
	// if home := os.Getenv("HOME"); home != "" {
	// 	viper.AddConfigPath("$HOME")
	// }
	// viper.AddConfigPath(".")
	// if err := viper.ReadInConfig(); err != nil {
	// 	// if _, ok := err.(viper.ConfigFileNotFoundError); ok {
	// 	if errors.Is(err, fs.ErrNotExist) {
	// 		return nil
	// 	}
	// 	return err
	// }
	return nil
}

type ConfigFileSettings map[string]any

func (cfs ConfigFileSettings) Set(key string, value any) {
	cfs[key] = value
}

func (cfs ConfigFileSettings) StoreConfig(configFile string) error {
	vip := viper.New()
	for k, v := range cfs {
		vip.Set(k, v)
	}
	vip.SetConfigFile(configFile)
	slog.Info("writing config file", "config-file", configFile)
	vip.SetConfigType("yaml")
	return vip.WriteConfig()
}
