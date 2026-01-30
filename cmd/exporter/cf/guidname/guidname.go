package guidname

import (
	"fmt"
	"regexp"

	"github.com/SAP/xp-clifford/erratt"
)

type Name struct {
	Name string
	GUID string
}

func NewName(guid, name string) *Name {
	return &Name{
		Name: name,
		GUID: guid,
	}
}

func (n *Name) String() string {
	return fmt.Sprintf("%s - [%s]", n.Name, n.GUID)
}

var nameRx = regexp.MustCompile(`(.*) - \[(.*)\]`)

func ParseName(s string) (*Name, error) {
	parsed := nameRx.FindStringSubmatch(s)
	switch len(parsed) {
	case 0:
		return &Name{
			Name: fmt.Sprintf("^%s$", s),
			GUID: s,
		}, nil
	case 3:
		return &Name{
			Name: fmt.Sprintf("^%s$", parsed[1]),
			GUID: parsed[2],
		}, nil
	default:
		return nil, erratt.New("guidname cannot be be parsed", "name", s, "len", len(parsed))
	}
}

func CollectNames(guidNames []string) ([]string, error) {
	names := make([]string, len(guidNames))
	for i, guidName := range guidNames {
		name, err := ParseName(guidName)
		if err != nil {
			return nil, err
		}
		names[i] = name.Name
	}
	return names, nil
}
