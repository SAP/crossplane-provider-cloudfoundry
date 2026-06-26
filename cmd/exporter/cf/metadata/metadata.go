// Package metadata provides shared helpers for the CF exporter.
package metadata

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

// StripDefaultLabels removes Crossplane default label keys from a label map.
// The default labels (crossplane-kind, crossplane-name, crossplane-providerconfig)
// are infrastructure metadata computed by the controller from the CR identity.
// They should not be included in exported ForProvider labels because they are
// semantically incorrect for a new CR (different name, different ProviderConfig)
// and the controller will recompute them on the next reconcile.
func StripDefaultLabels(labels map[string]*string) map[string]*string {
	if labels == nil {
		return nil
	}
	result := make(map[string]*string, len(labels))
	for k, v := range labels {
		if k == resource.ExternalResourceTagKeyKind ||
			k == resource.ExternalResourceTagKeyName ||
			k == resource.ExternalResourceTagKeyProvider {
			continue
		}
		result[k] = v
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
