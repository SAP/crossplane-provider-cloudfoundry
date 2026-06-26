// Package metadata provides shared helpers for managing Cloud Foundry
// resource metadata (labels and annotations) across the provider.
package metadata

import (
	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
)

// BuildMetadata constructs a *cfresource.Metadata for a Create or Update call
// by merging Crossplane default labels with user-specified labels and
// annotations.
//
// Default labels are obtained from resource.GetExternalTags(mg), which returns
// the three canonical keys (crossplane-kind, crossplane-name,
// crossplane-providerconfig). User-provided labels take precedence over
// defaults when keys collide.
//
// The mg parameter must implement resource.Managed so that GetExternalTags can
// extract the GVK, name, and ProviderConfig reference.
//
// Nil pointer values in userLabels and userAnnotations are treated as deletion
// markers per CF API convention: they are passed through to the resulting
// Metadata via RemoveLabel/RemoveAnnotation (setting the key's value to nil).
// On Create calls, nil values are no-ops (you cannot delete a label that does
// not yet exist). On Update calls, include nil values for keys you want to
// explicitly remove from the CF resource.
func BuildMetadata(mg resource.Managed, userLabels, userAnnotations map[string]*string) *cfresource.Metadata {
	var tags map[string]string
	if mg != nil {
		tags = resource.GetExternalTags(mg)
	}
	m := cfresource.NewMetadata()

	// Set default labels from Crossplane (map[string]string -> map[string]*string)
	for k, v := range tags {
		m.SetLabel("", k, v)
	}

	// Merge user labels (override defaults on collision)
	for k, v := range userLabels {
		if v == nil {
			m.RemoveLabel("", k)
			continue
		}
		m.SetLabel("", k, *v)
	}

	// Set user annotations
	for k, v := range userAnnotations {
		if v == nil {
			m.RemoveAnnotation("", k)
			continue
		}
		m.SetAnnotation("", k, *v)
	}

	return m
}

// MetadataMapEqual reports whether two metadata maps (labels or annotations)
// are semantically equal. nil and empty maps are considered equal. Pointer
// values are dereferenced for comparison; nil pointers indicate deletion
// markers and are treated as a distinct value (not equal to a non-nil pointer
// to an empty string).
func MetadataMapEqual(desired, actual map[string]*string) bool {
	if len(desired) == 0 && len(actual) == 0 {
		return true
	}
	if len(desired) != len(actual) {
		return false
	}
	for key, desiredVal := range desired {
		actualVal, exists := actual[key]
		if !exists {
			return false
		}
		if (desiredVal == nil) != (actualVal == nil) {
			return false
		}
		if desiredVal != nil && actualVal != nil && *desiredVal != *actualVal {
			return false
		}
	}
	return true
}

// MetadataMapContains reports whether all keys in desired are present and
// match in actual. Extra keys in actual that are not in desired are ignored.
// This implements a subset check: desired ⊆ actual.
//
// Use MetadataMapContains (not MetadataMapEqual) when checking whether a CF
// resource's metadata is up-to-date with the desired state, because the CF
// resource may have extra labels or annotations set by the platform or other
// actors that the provider does not manage.
func MetadataMapContains(desired, actual map[string]*string) bool {
	if len(desired) == 0 {
		return true
	}
	for key, desiredVal := range desired {
		actualVal, exists := actual[key]
		if !exists {
			if desiredVal == nil {
				continue
			}
			return false
		}
		if (desiredVal == nil) != (actualVal == nil) {
			return false
		}
		if desiredVal != nil && actualVal != nil && *desiredVal != *actualVal {
			return false
		}
	}
	return true
}

// IsMetadataUpToDate reports whether labels and annotations are in sync
// between the desired state and the actual state of the CF resource.
// It returns true only when every key in desiredLabels/desiredAnnotations
// is present and equal in the corresponding actual map. Extra keys in actual
// that are not in desired are ignored (they may be set by the CF platform or
// other actors).
//
// Callers should pass the full desired set (from BuildMetadata, which
// includes Crossplane default labels) as desiredLabels/desiredAnnotations,
// not just the CR spec's user labels.
func IsMetadataUpToDate(desiredLabels, desiredAnnotations, actualLabels, actualAnnotations map[string]*string) bool {
	return MetadataMapContains(desiredLabels, actualLabels) && MetadataMapContains(desiredAnnotations, actualAnnotations)
}

// diffMap computes the diff for a single metadata map (labels or annotations).
// It only processes keys that are present in desired:
//   - Keys in desired that are missing or different in actual are included
//   - Keys with nil pointer values in desired are included as deletion markers
//     only when the key exists in actual
//   - Keys in actual but absent from desired are NOT included (left unchanged
//     on the CF server, per merge-patch convention)
//
// Returns nil if there are no changes.
func diffMap(desired, actual map[string]*string) map[string]*string {
	if len(desired) == 0 {
		return nil
	}
	result := make(map[string]*string, len(desired))
	for k, desiredVal := range desired {
		actualVal, exists := actual[k]
		if !exists {
			if desiredVal == nil {
				continue
			}
			result[k] = desiredVal
			continue
		}
		if (desiredVal == nil) != (actualVal == nil) {
			result[k] = desiredVal
			continue
		}
		if desiredVal != nil && actualVal != nil && *desiredVal != *actualVal {
			result[k] = desiredVal
		}
		// identical: skip
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// DiffMetadata computes the metadata diff needed for an Update call. It
// returns nil if no metadata changes are needed, otherwise a *cfresource.Metadata
// containing only the keys that need to change.
//
// The diff follows CF API merge-patch semantics:
//   - Keys in desiredLabels/desiredAnnotations that are new or different from
//     actual are included (add/update)
//   - Keys with nil pointer values in desired are included as deletion markers
//   - Keys in actual but absent from desired are NOT included (left unchanged
//     on the CF server)
//
// Callers that want to delete a key that exists on the CF resource must
// include it in the desired map with a nil pointer value. For example, to
// remove label "env" from a CF resource, set desiredLabels["env"] = nil.
//
// Important: BuildMetadata always includes Crossplane default labels
// (crossplane-kind, crossplane-name, crossplane-providerconfig) in the
// desired set, so they are automatically maintained on every Update. Callers
// must ensure desiredLabels and desiredAnnotations come from BuildMetadata
// (or an equivalent merge) to avoid accidentally reverting default labels.
func DiffMetadata(desiredLabels, desiredAnnotations, actualLabels, actualAnnotations map[string]*string) *cfresource.Metadata {
	labels := diffMap(desiredLabels, actualLabels)
	annotations := diffMap(desiredAnnotations, actualAnnotations)
	if len(labels) == 0 && len(annotations) == 0 {
		return nil
	}

	m := cfresource.NewMetadata()
	m.Labels = labels
	m.Annotations = annotations
	return m
}

// StripDefaultLabels removes Crossplane default label keys from a label map.
// The default labels (crossplane-kind, crossplane-name, crossplane-providerconfig)
// are infrastructure metadata computed by the controller from the CR identity.
// They should not be late-initialized into spec.ForProvider.Labels because
// the controller recomputes them on every reconcile via BuildMetadata.
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
