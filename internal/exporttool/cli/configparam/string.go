package configparam

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type StringParam struct {
	*paramName
	defaultValue string
	sensitive    bool
}

var _ ConfigParam = &StringParam{}

func String(name, description string) *StringParam {
	return &StringParam{
		paramName:    newParamName(name, description),
		defaultValue: "",
		sensitive: false,
	}
}

func SensitiveString(name, description string) *StringParam {
	return &StringParam{
		paramName:    newParamName(name, description),
		defaultValue: "",
		sensitive: true,
	}
}

func (p *StringParam) WithDefaultValue(value string) *StringParam {
	p.defaultValue = value
	return p
}

func (p *StringParam) WithShortName(name string) ConfigParam {
	p.paramName.WithShortName(name)
	return p
}

func (p *StringParam) WithFlagName(name string) ConfigParam {
	p.paramName.WithFlagName(name)
	return p
}

func (p *StringParam) WithEnvVarName(name string) ConfigParam {
	p.paramName.WithEnvVarName(name)
	return p
}

func (p *StringParam) WithExample(example string) ConfigParam {
	p.paramName.WithExample(example)
	return p
}

func (p *StringParam) AttachToCommand(command *cobra.Command) {
	if p.paramName.ShortName != nil {
		command.PersistentFlags().StringP(p.FlagName, *p.ShortName, p.defaultValue, p.Description)
	} else {
		command.PersistentFlags().String(p.FlagName, p.defaultValue, p.Description)
	}
	if p.paramName.EnvVarName != "" {
		viper.BindEnv(p.FlagName, p.paramName.EnvVarName)
	}
	viper.BindPFlag(p.Name, command.PersistentFlags().Lookup(p.FlagName))
}

func (p *StringParam) ValueAsString() string {
	if p.sensitive {
		return "*****"
	} else {
		return p.paramName.ValueAsString()
	}
}

func (p *StringParam) Value() string {
	return viper.GetString(p.Name)
}

func (p *StringParam) ValueOrAsk() (string, error) {
	if p.paramName.IsSet() {
		return p.Value(), nil
	}
	value, err := p.askValue(p.sensitive)
	if err != nil {
		return "", err
	}
	viper.Set(p.Name, value)
	return value, nil
}
