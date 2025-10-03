package configparam

import (
	"fmt"

	"github.com/charmbracelet/huh"
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

func (p *paramName) askValue(sensitive bool) (string, error) {
	var value string
	echoMode := huh.EchoModeNormal
	if sensitive {
		echoMode = huh.EchoModePassword
	}
	err := huh.NewInput().
		Value(&value).
		Title(p.inputPrompt()).
		Placeholder(p.Example).
		EchoMode(echoMode).
		Run()
	return value, err
}

type ConfigParam interface {
	GetName() string
	AttachToCommand(cmd *cobra.Command)
	BindConfiguration(cmd *cobra.Command)
	ValueAsString() string
	IsSet() bool
}

// type GlobalConfigParam struct {
// 	ConfigParam
// }

// func NewGlobalBoolConfigParam(name, description string) *GlobalConfigParam {
// 	return &GlobalConfigParam{
// 		ConfigParam: *NewBoolConfigParam(name, description),
// 	}
// }

// func (p *GlobalConfigParam) WithShortName(shortName string) *GlobalConfigParam {
// 	p.ConfigParam.WithShortName(shortName)
// 	return p
// }

// func (p *GlobalConfigParam) WithDefaultValue(value any) *GlobalConfigParam {
// 	p.ConfigParam.WithDefaultValue(value)
// 	return p
// }

// func (p *GlobalConfigParam) Configure(command *cobra.Command) {
// 	p.ConfigParam.ForCommand(command)
// }
