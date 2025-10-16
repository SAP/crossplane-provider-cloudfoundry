package configparam

import (
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/widget"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type possibleValuesFnType func() ([]string, error)

type StringSliceParam struct {
	*paramName
	defaultValue     []string
	sensitive        bool
	possibleValues   []string
	possibleValuesFn possibleValuesFnType
}

var _ ConfigParam = &StringSliceParam{}

// func basicValueToPair(ss []string) [][2]string {
// 	result := make([][2]string, len(ss))
// 	for i, s := range ss {
// 		result[i][0] = s
// 		result[i][1] = s
// 	}
// 	return result
// }

func StringSlice(name, description string) *StringSliceParam {
	return &StringSliceParam{
		paramName:      newParamName(name, description),
		defaultValue:   []string{},
		sensitive:      false,
		possibleValues: []string{},
	}
}

func SensitiveStringSlice(name, description string) *StringSliceParam {
	return &StringSliceParam{
		paramName:    newParamName(name, description),
		defaultValue: []string{},
		sensitive:    true,
	}
}

func (p *StringSliceParam) WithDefaultValue(values []string) *StringSliceParam {
	p.defaultValue = values
	return p
}

func (p *StringSliceParam) WithPossibleValues(values []string) *StringSliceParam {
	p.possibleValues = values
	return p
}

// func toPairFn(fn func() ([]string, error)) possibleValuesFnType {
// 	return func() ([][2]string, error) {
// 		values, err := fn()
// 		if err != nil {
// 			return nil, err
// 		}
// 		pairValues := make([][2]string, len(values))
// 		for i, v := range values {
// 			pairValues[i][0] = v
// 			pairValues[i][1] = v
// 		}
// 		return pairValues, nil
// 	}
// }

func (p *StringSliceParam) WithShortName(name string) *StringSliceParam {
	p.paramName.WithShortName(name)
	return p
}

func (p *StringSliceParam) WithFlagName(name string) *StringSliceParam {
	p.paramName.WithFlagName(name)
	return p
}

func (p *StringSliceParam) WithPossibleValuesFn(fn func() ([]string, error)) *StringSliceParam {
	p.possibleValuesFn = fn
	return p
}

// func (p *StringSliceParam) WithPossibleValuesPairFn(fn possibleValuesFnType) *StringSliceParam {
// 	p.possibleValuesFn = fn
// 	return p
// }

func (p *StringSliceParam) WithEnvVarName(name string) *StringSliceParam {
	p.paramName.WithEnvVarName(name)
	return p
}

func (p *StringSliceParam) WithExample(example string) *StringSliceParam {
	p.paramName.WithExample(example)
	return p
}

func (p *StringSliceParam) AttachToCommand(command *cobra.Command) {
	if p.paramName.ShortName != nil {
		command.PersistentFlags().StringSliceP(p.FlagName, *p.ShortName, p.defaultValue, p.Description)
	} else {
		command.PersistentFlags().StringSlice(p.FlagName, p.defaultValue, p.Description)
	}
	if p.paramName.EnvVarName != "" {
		if err := viper.BindEnv(p.Name, p.paramName.EnvVarName); err != nil {
			panic(err)
		}
	}
	if err := viper.BindPFlag(p.Name, command.PersistentFlags().Lookup(p.FlagName)); err != nil {
		panic(err)
	}
}

func (p *StringSliceParam) BindConfiguration(command *cobra.Command) {
	if p.paramName.EnvVarName != "" {
		if err := viper.BindEnv(p.Name, p.paramName.EnvVarName); err != nil {
			panic(err)
		}
	}
	if err := viper.BindPFlag(p.Name, command.PersistentFlags().Lookup(p.FlagName)); err != nil {
		panic(err)
	}
}

func (p *StringSliceParam) ValueAsString() string {
	if p.sensitive {
		return "*****"
	} else {
		return p.paramName.ValueAsString()
	}
}

func (p *StringSliceParam) Value() []string {
	return viper.GetStringSlice(p.Name)
}

func (p *StringSliceParam) ValueOrAsk() ([]string, error) {
	if p.paramName.IsSet() {
		return p.Value(), nil
	}
	if len(p.possibleValues) == 0 && p.possibleValuesFn == nil {
		return nil, erratt.New("StringSliceParam ValueOrAsk invoked but possibleValues are not set", "name", p.paramName.Name)
	}
	possibleValues := p.possibleValues
	if len(possibleValues) == 0 {
		var err error
		possibleValues, err = p.possibleValuesFn()
		if err != nil {
			return nil, erratt.Errorf("cannot get possible values: %w", err)
		}
	}
	values := widget.MultiInput(p.paramName.Description,
		possibleValues,
	)
	viper.Set(p.Name, values)
	return values, nil
}
