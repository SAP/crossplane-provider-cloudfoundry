package configparam

import (
	"context"

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
		sensitive:    false,
	}
}

func SensitiveString(name, description string) *StringParam {
	return &StringParam{
		paramName:    newParamName(name, description),
		defaultValue: "",
		sensitive:    true,
	}
}

func (p *StringParam) WithDefaultValue(value string) *StringParam {
	p.defaultValue = value
	return p
}

func (p *StringParam) WithShortName(name string) *StringParam {
	p.paramName.WithShortName(name)
	return p
}

func (p *StringParam) WithFlagName(name string) *StringParam {
	p.paramName.WithFlagName(name)
	return p
}

func (p *StringParam) WithEnvVarName(name string) *StringParam {
	p.paramName.WithEnvVarName(name)
	if err := viper.BindEnv(p.Name, name); err != nil {
		panic(err)
	}
	return p
}

func (p *StringParam) WithExample(example string) *StringParam {
	p.paramName.WithExample(example)
	return p
}

func (p *StringParam) AttachToCommand(command *cobra.Command) {
	if p.paramName.ShortName != nil {
		command.PersistentFlags().StringP(p.FlagName, *p.ShortName, p.defaultValue, p.Description)
	} else {
		command.PersistentFlags().String(p.FlagName, p.defaultValue, p.Description)
	}
}

func (p *StringParam) BindConfiguration(command *cobra.Command) {
	if p.paramName.EnvVarName != "" {
		if err := viper.BindEnv(p.Name, p.paramName.EnvVarName); err != nil {
			panic(err)
		}
	}
	if err := viper.BindPFlag(p.Name, command.PersistentFlags().Lookup(p.FlagName)); err != nil {
		panic(err)
	}
}

func (p *StringParam) ValueAsString() string {
	if p.sensitive {
		return "*****"
	} else {
		return p.paramName.ValueAsString()
	}
}

func (p *StringParam) Value() string {
	if p.paramName.IsSet() {
		return viper.GetString(p.Name)
	} else {
		return p.defaultValue
	}
}

func (p *StringParam) ValueOrAsk(ctx context.Context) (string, error) {
	if p.paramName.IsSet() {
		return p.Value(), nil
	}
	return p.AskValue(ctx)
}

func (p *StringParam) AskValue(ctx context.Context) (string, error) {
	value, err := p.askValue(ctx, p.sensitive)
	if err != nil {
		return "", err
	}
	viper.Set(p.Name, value)
	return value, nil
}
