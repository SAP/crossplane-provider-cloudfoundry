package cli

import (
	"reflect"
	"runtime"
	"strings"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"
)

type ConfigSchema struct {
	CLIConfiguration
	// ExportSubcommandConfiguration
}

type ConfigModule func() error

var (
	Configuration *ConfigSchema = &ConfigSchema{}
	configModules               = []ConfigModule{}
)

func RegisterConfigModule(module ConfigModule) {
	configModules = append(configModules, module)
}

func configModuleName(module ConfigModule) string {
	s := strings.Split(runtime.FuncForPC(reflect.ValueOf(module).Pointer()).Name(), ".")
	if len(s) == 0 {
		panic("invalid configModule name")
	}
	return s[len(s)-1]
}

func configure() error {
	for _, fn := range configModules {
		if err := fn(); err != nil {
			return erratt.Errorf("configuration failed: %w", err).
				With("module", configModuleName(fn))
		}
	}
	return nil
}
