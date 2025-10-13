package yaml

import (
	"fmt"

	"github.com/charmbracelet/glamour"
	"sigs.k8s.io/yaml"
)

func Marshal(resource any) (string, error) {
	b, err := yaml.Marshal(resource)
	if err != nil {
		return "", err
	}
	return glamour.Render(fmt.Sprintf("```yaml\n---\n%s...\n```", string(b)), "auto")
	// return string(b), nil
}
