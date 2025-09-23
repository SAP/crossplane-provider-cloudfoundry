package subcommand

import (
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"
)

type SubCommand interface {
	GetName() string
	GetShort() string
	GetLong() string
	GetConfigParams() configparam.ParamList
	Run() func() error
}

type Simple struct {
	Name         string
	Short        string
	Long         string
	ConfigParams configparam.ParamList
	Logic        func() error
}

var _ SubCommand = &Simple{}

func (s *Simple) GetName() string {
	return s.Name
}

func (s *Simple) GetShort() string {
	return s.Short
}

func (s *Simple) GetLong() string {
	return s.Long
}

func (s *Simple) GetConfigParams() configparam.ParamList {
	return s.ConfigParams
}

func (s *Simple) Run() func() error {
	return s.Logic
}
