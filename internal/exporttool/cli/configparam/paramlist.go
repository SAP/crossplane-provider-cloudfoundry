package configparam

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

func boolToYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func configParamToRow(cp ConfigParam) table.Row {
	return table.Row{
		cp.GetName(),
		cp.ValueAsString(),
		boolToYesNo(cp.IsSet()),
	}
}

type ParamList []ConfigParam

func (pl ParamList) toRows() []table.Row {
	rows := make([]table.Row, len(pl))

	for i := range pl {
		rows[i] = configParamToRow(pl[i])
	}
	return rows
}

var tableStyle = table.Styles{
	Selected: lipgloss.NewStyle(),
	Header:   lipgloss.NewStyle().Bold(true).Padding(0, 1),
	Cell:     lipgloss.NewStyle().Padding(0, 1),
}

func (pl ParamList) String() string {
	t := table.New()
	t.SetColumns([]table.Column{
		{
			Title: "Parameter",
			Width: 15,
		},
		{
			Title: "Value",
			Width: 15,
		},
		{
			Title: "Configured?",
			Width: 15,
		},
	})
	t.SetRows(pl.toRows())
	t.SetStyles(tableStyle)
	return t.View()
}
