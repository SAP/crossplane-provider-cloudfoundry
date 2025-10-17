package configparam

import (
	"context"
	"fmt"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/widget"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type paramName struct {
	Name        string
	Description string
	FlagName    string
	ShortName   *string
	EnvVarName  string
	Example     string
}

func newParamName(name, description string) *paramName {
	return &paramName{
		Name:        name,
		Description: description,
		FlagName:    name,
		EnvVarName:  name,
	}
}

func (p *paramName) GetName() string {
	return p.Name
}

func (p *paramName) ValueAsString() string {
	return viper.GetString(p.Name)
}

func (p *paramName) WithShortName(name string) *paramName {
	p.ShortName = &name
	return p
}

func (p *paramName) WithFlagName(name string) *paramName {
	p.FlagName = name
	return p
}

func (p *paramName) WithEnvVarName(name string) *paramName {
	p.EnvVarName = name
	return p
}

func (p *paramName) WithExample(example string) *paramName {
	p.Example = example
	return p
}

func (p *paramName) IsSet() bool {
	return viper.IsSet(p.Name)
}

func (p *paramName) inputPrompt() string {
	return fmt.Sprintf("%s [%s]: ", p.Description, p.Name)
}

func (p *paramName) askValue(ctx context.Context, sensitive bool) (string, error) {
	return widget.TextInput(ctx,
		p.inputPrompt(),
		p.Example,
		sensitive,
	)
}

type ConfigParam interface {
	GetName() string
	AttachToCommand(cmd *cobra.Command)
	BindConfiguration(cmd *cobra.Command)
	ValueAsString() string
	IsSet() bool
}
