// Package metadata provides shared helpers for the CF exporter.
package metadata

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

// StripDefaultTags removes the Crossplane default tag keys from CF labels or
// annotations. They identify the source CR, so they don't belong on an
// exported one. Returns nil if no keys remain.
func StripDefaultTags(m map[string]*string) map[string]*string {
	if m == nil {
		return nil
	}
	result := make(map[string]*string, len(m))
	for k, v := range m {
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
