package metadata

import (
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"

	v1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

func newTestManaged(name, providerCfg string) *v1alpha1.Space {
	s := &v1alpha1.Space{}
	s.SetName(name)
	if providerCfg != "" {
		s.SetProviderConfigReference(&xpv1.Reference{Name: providerCfg})
	}
	return s
}

func ptrTo(s string) *string { return &s }

func TestBuildMetadata(t *testing.T) {
	t.Parallel()

	t.Run("defaults only - no user labels or annotations", func(t *testing.T) {
		mg := newTestManaged("my-space", "my-config")
		m := BuildMetadata(mg, nil, nil)

		if m == nil {
			t.Fatal("expected non-nil metadata")
		}
		if len(m.Labels) != 3 {
			t.Fatalf("expected 3 default labels, got %d: %v", len(m.Labels), m.Labels)
		}
		if v := m.Labels["crossplane-name"]; v == nil || *v != "my-space" {
			t.Errorf("expected crossplane-name=my-space, got %v", v)
		}
		if v := m.Labels["crossplane-providerconfig"]; v == nil || *v != "my-config" {
			t.Errorf("expected crossplane-providerconfig=my-config, got %v", v)
		}
		if _, ok := m.Labels["crossplane-kind"]; !ok {
			t.Error("expected crossplane-kind label to be present")
		}
		if len(m.Annotations) != 0 {
			t.Errorf("expected no annotations, got %d", len(m.Annotations))
		}
	})

	t.Run("defaults plus user labels", func(t *testing.T) {
		mg := newTestManaged("my-space", "my-config")
		m := BuildMetadata(mg, map[string]*string{"env": ptrTo("production")}, nil)

		if len(m.Labels) != 4 {
			t.Fatalf("expected 4 labels (3 default + 1 user), got %d", len(m.Labels))
		}
		if v := m.Labels["env"]; v == nil || *v != "production" {
			t.Errorf("expected env=production, got %v", v)
		}
	})

	t.Run("user labels override defaults on collision", func(t *testing.T) {
		mg := newTestManaged("my-space", "my-config")
		m := BuildMetadata(mg, map[string]*string{"crossplane-name": ptrTo("override-name")}, nil)

		if v := m.Labels["crossplane-name"]; v == nil || *v != "override-name" {
			t.Errorf("expected crossplane-name=override-name, got %v", v)
		}
	})

	t.Run("no provider config ref - crossplane-providerconfig omitted", func(t *testing.T) {
		mg := newTestManaged("my-space", "")
		m := BuildMetadata(mg, nil, nil)

		if len(m.Labels) != 2 {
			t.Fatalf("expected 2 labels (no providerconfig), got %d: %v", len(m.Labels), m.Labels)
		}
		if _, ok := m.Labels["crossplane-providerconfig"]; ok {
			t.Error("expected crossplane-providerconfig to be absent")
		}
	})

	t.Run("defaults plus user labels and annotations", func(t *testing.T) {
		mg := newTestManaged("my-space", "my-config")
		m := BuildMetadata(mg,
			map[string]*string{"env": ptrTo("staging")},
			map[string]*string{"description": ptrTo("my test space")},
		)

		if len(m.Labels) != 4 {
			t.Fatalf("expected 4 labels, got %d", len(m.Labels))
		}
		if len(m.Annotations) != 1 {
			t.Fatalf("expected 1 annotation, got %d", len(m.Annotations))
		}
		if v := m.Annotations["description"]; v == nil || *v != "my test space" {
			t.Errorf("expected description='my test space', got %v", v)
		}
	})

	t.Run("nil pointer values in userLabels produce deletion markers", func(t *testing.T) {
		mg := newTestManaged("my-space", "my-config")
		m := BuildMetadata(mg,
			map[string]*string{"stale-key": nil},
			map[string]*string{"stale-annotation": nil},
		)

		if v, ok := m.Labels["stale-key"]; !ok {
			t.Error("expected stale-key to be present as deletion marker")
		} else if v != nil {
			t.Errorf("expected nil deletion marker for stale-key, got %v", v)
		}
		if v, ok := m.Annotations["stale-annotation"]; !ok {
			t.Error("expected stale-annotation to be present as deletion marker")
		} else if v != nil {
			t.Errorf("expected nil deletion marker for stale-annotation, got %v", v)
		}
		if len(m.Labels) != 4 {
			t.Errorf("expected 4 labels (3 default + 1 deletion marker), got %d: %v", len(m.Labels), m.Labels)
		}
	})

	t.Run("nil pointer value overrides default label", func(t *testing.T) {
		mg := newTestManaged("my-space", "my-config")
		m := BuildMetadata(mg, map[string]*string{"crossplane-name": nil}, nil)

		if v, ok := m.Labels["crossplane-name"]; !ok {
			t.Error("expected crossplane-name to be present")
		} else if v != nil {
			t.Errorf("expected nil deletion marker for crossplane-name, got %v", v)
		}
	})

	t.Run("nil managed resource - no default labels", func(t *testing.T) {
		m := BuildMetadata(nil, nil, nil)

		if m == nil {
			t.Fatal("expected non-nil metadata")
		}
		if len(m.Labels) != 0 {
			t.Fatalf("expected 0 labels with nil mg, got %d", len(m.Labels))
		}
		if len(m.Annotations) != 0 {
			t.Fatalf("expected 0 annotations with nil mg, got %d", len(m.Annotations))
		}
	})
}

func TestBuildMetadata_ProducesValidCFMetadata(t *testing.T) {
	mg := newTestManaged("test-space", "test-config")
	m := BuildMetadata(mg,
		map[string]*string{"env": ptrTo("prod")},
		map[string]*string{"note": ptrTo("test")},
	)

	if len(m.Labels) != 4 {
		t.Fatalf("expected 4 labels (3 default + 1 user), got %d", len(m.Labels))
	}
	if len(m.Annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(m.Annotations))
	}
}

func TestMetadataMapEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		desired map[string]*string
		actual  map[string]*string
		want    bool
	}{
		{"both nil", nil, nil, true},
		{"both empty", map[string]*string{}, map[string]*string{}, true},
		{"nil and empty", nil, map[string]*string{}, true},
		{"same single key", map[string]*string{"key": ptrTo("value")}, map[string]*string{"key": ptrTo("value")}, true},
		{"different values", map[string]*string{"key": ptrTo("a")}, map[string]*string{"key": ptrTo("b")}, false},
		{"missing key in actual", map[string]*string{"key": ptrTo("a"), "extra": ptrTo("b")}, map[string]*string{"key": ptrTo("a")}, false},
		{"extra key in actual", map[string]*string{"key": ptrTo("a")}, map[string]*string{"key": ptrTo("a"), "extra": ptrTo("b")}, false},
		{"nil pointer vs nil pointer", map[string]*string{"key": nil}, map[string]*string{"key": nil}, true},
		{"nil pointer vs non-nil pointer", map[string]*string{"key": nil}, map[string]*string{"key": ptrTo("")}, false},
		{"non-nil pointer vs nil pointer", map[string]*string{"key": ptrTo("val")}, map[string]*string{"key": nil}, false},
		{"multiple matching keys", map[string]*string{"a": ptrTo("1"), "b": ptrTo("2")}, map[string]*string{"a": ptrTo("1"), "b": ptrTo("2")}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MetadataMapEqual(tt.desired, tt.actual); got != tt.want {
				t.Errorf("MetadataMapEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetadataMapContains(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		desired map[string]*string
		actual  map[string]*string
		want    bool
	}{
		{"both nil", nil, nil, true},
		{"desired nil actual has keys", nil, map[string]*string{"key": ptrTo("value")}, true},
		{"desired empty actual has keys", map[string]*string{}, map[string]*string{"key": ptrTo("value")}, true},
		{"exact match", map[string]*string{"key": ptrTo("value")}, map[string]*string{"key": ptrTo("value")}, true},
		{"desired subset of actual", map[string]*string{"key": ptrTo("value")}, map[string]*string{"key": ptrTo("value"), "extra": ptrTo("data")}, true},
		{"desired key missing from actual", map[string]*string{"key": ptrTo("value"), "missing": ptrTo("data")}, map[string]*string{"key": ptrTo("value")}, false},
		{"desired value differs from actual", map[string]*string{"key": ptrTo("new")}, map[string]*string{"key": ptrTo("old"), "extra": ptrTo("data")}, false},
		{"nil pointer deletion marker match", map[string]*string{"key": nil}, map[string]*string{"key": nil, "extra": ptrTo("data")}, true},
		{"nil pointer deletion marker already absent", map[string]*string{"key": nil}, map[string]*string{"extra": ptrTo("data")}, true},
		{"nil pointer vs non-nil pointer", map[string]*string{"key": nil}, map[string]*string{"key": ptrTo("")}, false},
		{"non-nil pointer vs nil pointer in actual", map[string]*string{"key": ptrTo("val")}, map[string]*string{"key": nil, "extra": ptrTo("data")}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MetadataMapContains(tt.desired, tt.actual); got != tt.want {
				t.Errorf("MetadataMapContains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsMetadataUpToDate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		desiredLabels      map[string]*string
		desiredAnnotations map[string]*string
		actualLabels       map[string]*string
		actualAnnotations  map[string]*string
		want               bool
	}{
		{"all nil", nil, nil, nil, nil, true},
		{"labels match annotations nil", map[string]*string{"key": ptrTo("val")}, nil, map[string]*string{"key": ptrTo("val")}, nil, true},
		{"labels match annotations mismatch", map[string]*string{"key": ptrTo("val")}, map[string]*string{"note": ptrTo("a")}, map[string]*string{"key": ptrTo("val")}, map[string]*string{"note": ptrTo("b")}, false},
		{"labels mismatch", map[string]*string{"key": ptrTo("a")}, nil, map[string]*string{"key": ptrTo("b")}, nil, false},
		{"both match", map[string]*string{"key": ptrTo("val")}, map[string]*string{"note": ptrTo("a")}, map[string]*string{"key": ptrTo("val")}, map[string]*string{"note": ptrTo("a")}, true},
		{"actual has extra keys", map[string]*string{"key": ptrTo("val")}, nil, map[string]*string{"key": ptrTo("val"), "system-label": ptrTo("system-val")}, map[string]*string{"system-annotation": ptrTo("data")}, true},
		{"desired key missing from actual", map[string]*string{"key": ptrTo("val"), "missing": ptrTo("data")}, nil, map[string]*string{"key": ptrTo("val")}, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsMetadataUpToDate(tt.desiredLabels, tt.desiredAnnotations, tt.actualLabels, tt.actualAnnotations); got != tt.want {
				t.Errorf("IsMetadataUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiffMetadata(t *testing.T) {
	t.Parallel()

	t.Run("no diff - identical maps", func(t *testing.T) {
		m := DiffMetadata(map[string]*string{"a": ptrTo("1"), "b": ptrTo("2")}, nil, map[string]*string{"a": ptrTo("1"), "b": ptrTo("2")}, nil)
		if m != nil {
			t.Errorf("expected nil diff, got labels=%v annotations=%v", m.Labels, m.Annotations)
		}
	})

	t.Run("add new key", func(t *testing.T) {
		m := DiffMetadata(map[string]*string{"a": ptrTo("1"), "b": ptrTo("2")}, nil, map[string]*string{"a": ptrTo("1")}, nil)
		if len(m.Labels) != 1 {
			t.Fatalf("expected 1 key in diff, got %d", len(m.Labels))
		}
		if v := m.Labels["b"]; v == nil || *v != "2" {
			t.Errorf("expected b=2, got %v", v)
		}
	})

	t.Run("update existing key", func(t *testing.T) {
		m := DiffMetadata(map[string]*string{"a": ptrTo("new")}, nil, map[string]*string{"a": ptrTo("old")}, nil)
		if len(m.Labels) != 1 {
			t.Fatalf("expected 1 key in diff, got %d", len(m.Labels))
		}
		if v := m.Labels["a"]; v == nil || *v != "new" {
			t.Errorf("expected a=new, got %v", v)
		}
	})

	t.Run("keys in actual but not in desired are left alone", func(t *testing.T) {
		m := DiffMetadata(map[string]*string{"a": ptrTo("1")}, nil, map[string]*string{"a": ptrTo("1"), "system-label": ptrTo("system-val")}, nil)
		if m != nil {
			t.Errorf("expected nil diff, got labels=%v annotations=%v", m.Labels, m.Annotations)
		}
	})

	t.Run("explicit nil in desired produces deletion marker", func(t *testing.T) {
		m := DiffMetadata(map[string]*string{"a": ptrTo("1"), "stale": nil}, nil, map[string]*string{"a": ptrTo("1"), "stale": ptrTo("old-val")}, nil)
		if len(m.Labels) != 1 {
			t.Fatalf("expected 1 key in diff, got %d: %v", len(m.Labels), m.Labels)
		}
		if v, ok := m.Labels["stale"]; !ok || v != nil {
			t.Errorf("expected nil deletion marker for stale, got %v", v)
		}
	})

	t.Run("both nil maps", func(t *testing.T) {
		if m := DiffMetadata(nil, nil, nil, nil); m != nil {
			t.Errorf("expected nil diff, got labels=%v annotations=%v", m.Labels, m.Annotations)
		}
	})

	t.Run("nil pointer deletion marker already absent produces no diff", func(t *testing.T) {
		if m := DiffMetadata(map[string]*string{"a": nil}, nil, map[string]*string{}, nil); m != nil {
			t.Errorf("expected nil diff, got labels=%v annotations=%v", m.Labels, m.Annotations)
		}
	})

	t.Run("nil pointer value differs from non-nil", func(t *testing.T) {
		m := DiffMetadata(map[string]*string{"a": nil}, nil, map[string]*string{"a": ptrTo("value")}, nil)
		if len(m.Labels) != 1 {
			t.Fatalf("expected 1 key in diff, got %d", len(m.Labels))
		}
		if v := m.Labels["a"]; v != nil {
			t.Errorf("expected nil value in diff, got %v", v)
		}
	})

	t.Run("combined add and update", func(t *testing.T) {
		m := DiffMetadata(map[string]*string{"a": ptrTo("1"), "c": ptrTo("3")}, nil, map[string]*string{"a": ptrTo("old"), "b": ptrTo("2")}, nil)
		if len(m.Labels) != 2 {
			t.Fatalf("expected 2 keys in diff, got %d: %v", len(m.Labels), m.Labels)
		}
		if v := m.Labels["a"]; v == nil || *v != "1" {
			t.Errorf("expected a=1, got %v", v)
		}
		if v := m.Labels["c"]; v == nil || *v != "3" {
			t.Errorf("expected c=3, got %v", v)
		}
		if _, ok := m.Labels["b"]; ok {
			t.Error("expected b NOT to be in diff")
		}
	})
}

func TestDiffMetadata_Annotations(t *testing.T) {
	t.Parallel()

	t.Run("annotation diff only", func(t *testing.T) {
		m := DiffMetadata(
			map[string]*string{"a": ptrTo("1")},
			map[string]*string{"note": ptrTo("updated")},
			map[string]*string{"a": ptrTo("1")},
			map[string]*string{"note": ptrTo("old"), "extra": ptrTo("data")},
		)
		if len(m.Labels) != 0 {
			t.Errorf("expected empty label diff, got %d keys", len(m.Labels))
		}
		if len(m.Annotations) != 1 {
			t.Fatalf("expected 1 annotation in diff, got %d", len(m.Annotations))
		}
		if v := m.Annotations["note"]; v == nil || *v != "updated" {
			t.Errorf("expected note=updated, got %v", v)
		}
	})

	t.Run("both label and annotation diffs", func(t *testing.T) {
		m := DiffMetadata(
			map[string]*string{"a": ptrTo("1"), "new": ptrTo("val")},
			map[string]*string{"note": nil},
			map[string]*string{"a": ptrTo("old")},
			map[string]*string{"note": ptrTo("stale")},
		)
		if len(m.Labels) != 2 {
			t.Fatalf("expected 2 label diffs, got %d: %v", len(m.Labels), m.Labels)
		}
		if len(m.Annotations) != 1 {
			t.Fatalf("expected 1 annotation diff, got %d", len(m.Annotations))
		}
		if v := m.Annotations["note"]; v != nil {
			t.Errorf("expected nil deletion marker for note, got %v", v)
		}
	})

	t.Run("no annotations desired - actual annotations ignored", func(t *testing.T) {
		m := DiffMetadata(
			map[string]*string{"a": ptrTo("1")},
			nil,
			map[string]*string{"a": ptrTo("1")},
			map[string]*string{"system": ptrTo("data")},
		)
		if m != nil {
			t.Errorf("expected nil diff, got labels=%v annotations=%v", m.Labels, m.Annotations)
		}
	})
}

func TestDiffMetadata_WithCrossplaneDefaultLabels(t *testing.T) {
	t.Parallel()

	t.Run("desired includes defaults actual has none", func(t *testing.T) {
		desired := map[string]*string{
			"crossplane-kind":           ptrTo("space.cloudfoundry.crossplane.io"),
			"crossplane-name":           ptrTo("my-space"),
			"crossplane-providerconfig": ptrTo("my-config"),
			"env":                       ptrTo("prod"),
		}
		m := DiffMetadata(desired, nil, map[string]*string{"env": ptrTo("prod")}, nil)
		if len(m.Labels) != 3 {
			t.Fatalf("expected 3 keys to add, got %d: %v", len(m.Labels), m.Labels)
		}
		for _, k := range []string{"crossplane-kind", "crossplane-name", "crossplane-providerconfig"} {
			if _, ok := m.Labels[k]; !ok {
				t.Errorf("expected %q in diff", k)
			}
		}
	})

	t.Run("actual has stale defaults desired has updated values", func(t *testing.T) {
		m := DiffMetadata(
			map[string]*string{"crossplane-name": ptrTo("new-space-name")},
			nil,
			map[string]*string{"crossplane-name": ptrTo("old-space-name")},
			nil,
		)
		if len(m.Labels) != 1 {
			t.Fatalf("expected 1 key in diff, got %d", len(m.Labels))
		}
		if v := m.Labels["crossplane-name"]; v == nil || *v != "new-space-name" {
			t.Errorf("expected crossplane-name=new-space-name, got %v", v)
		}
	})

	t.Run("actual has extra system labels - not in diff", func(t *testing.T) {
		m := DiffMetadata(
			map[string]*string{"crossplane-kind": ptrTo("space.cloudfoundry.crossplane.io"), "crossplane-name": ptrTo("my-space")},
			nil,
			map[string]*string{"crossplane-kind": ptrTo("space.cloudfoundry.crossplane.io"), "crossplane-name": ptrTo("my-space"), "cf-system-label": ptrTo("system-val")},
			nil,
		)
		if m != nil {
			t.Errorf("expected nil diff, got labels=%v annotations=%v", m.Labels, m.Annotations)
		}
	})
}
