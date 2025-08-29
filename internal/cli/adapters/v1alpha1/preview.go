package v1alpha1

import (
	"fmt"
	"log/slog"

	"github.com/charmbracelet/glamour"
	"sigs.k8s.io/yaml"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/provider"
)

func preview(resource provider.Resource) {
	y, err := yaml.Marshal(resource.GetManagedResource())
	if err != nil {
		slog.Error("cannot marshal to yaml", "error", err)
		return
	}
	s := fmt.Sprintf("## %s: %s\n```yaml\n%s\n```\n---\n",
		resource.GetResourceType(),
		resource.GetExternalID(),
		string(y))
	out, err := glamour.Render(s, "auto")
	if err != nil {
		slog.Error("render yaml to terminal", "error", err)
		return
	}
	fmt.Println(out)
}
