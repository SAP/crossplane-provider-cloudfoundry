package configparam

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type BoolParam struct {
	*paramName
	defaultValue bool
}

var _ ConfigParam = &BoolParam{}

func Bool(name, description string) *BoolParam {
	return &BoolParam{
		paramName:    newParamName(name, description),
		defaultValue: false,
	}
}

func (p *BoolParam) WithDefaultValue(value bool) *BoolParam {
	p.defaultValue = value
	return p
}

func (p *BoolParam) WithShortName(name string) ConfigParam {
	p.paramName.WithShortName(name)
	return p
}

func (p *BoolParam) WithFlagName(name string) ConfigParam {
	p.paramName.WithFlagName(name)
	return p
}

func (p *BoolParam) WithEnvVarName(name string) ConfigParam {
	p.paramName.WithEnvVarName(name)
	return p
}

func (p *BoolParam) WithExample(example string) ConfigParam {
	p.paramName.WithExample(example)
	return p
}

func (p *BoolParam) AttachToCommand(command *cobra.Command) {
	if p.paramName.ShortName != nil {
		command.PersistentFlags().BoolP(p.FlagName, *p.ShortName, p.defaultValue, p.Description)
	} else {
		command.PersistentFlags().Bool(p.FlagName, p.defaultValue, p.Description)
	}
	if p.paramName.EnvVarName != "" {
		viper.BindEnv(p.FlagName, p.paramName.EnvVarName)
	}
	viper.BindPFlag(p.Name, command.PersistentFlags().Lookup(p.FlagName))
}

// func (p *BoolParam) ValueAsString() string {
// 	return fmt.Sprintf("%t", p.Value())
// }

func (p *BoolParam) Value() bool {
	return viper.GetBool(p.Name)
}
