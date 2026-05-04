package metadata

import (
	"testing"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// mockManaged is a minimal resource.Managed implementation for testing.
type mockManaged struct {
	resource.Managed // embed to satisfy interface; override used methods
	name             string
	providerCfgRef   *xpv1.Reference
	gvk              schema.GroupVersionKind
}

func (m *mockManaged) GetName() string { return m.name }

func (m *mockManaged) GetProviderConfigReference() *xpv1.Reference {
	return m.providerCfgRef
}

func (m *mockManaged) GetObjectKind() schema.ObjectKind {
	return &objectKind{gvk: m.gvk}
}

type objectKind struct {
	gvk schema.GroupVersionKind
}

func (o *objectKind) SetGroupVersionKind(gvk schema.GroupVersionKind) { o.gvk = gvk }
func (o *objectKind) GroupVersionKind() schema.GroupVersionKind       { return o.gvk }

func newMockManaged(name, providerCfg string, gvk schema.GroupVersionKind) *mockManaged {
	m := &mockManaged{name: name, gvk: gvk}
	if providerCfg != "" {
		m.providerCfgRef = &xpv1.Reference{Name: providerCfg}
	}
	return m
}

func ptrTo(s string) *string { return &s }

func TestBuildMetadata(t *testing.T) {
	spaceGVK := schema.GroupVersionKind{
		Group:   "cloudfoundry.crossplane.io",
		Version: "v1alpha1",
		Kind:    "Space",
	}

	t.Parallel()

	t.Run("defaults only - no user labels or annotations", func(t *testing.T) {
		mg := newMockManaged("my-space", "my-config", spaceGVK)
		m := BuildMetadata(mg, nil, nil)

		if m == nil {
			t.Fatal("expected non-nil metadata")
		}
		if len(m.Labels) != 3 {
			t.Fatalf("expected 3 default labels, got %d", len(m.Labels))
		}
		if v := m.Labels["crossplane-kind"]; v == nil || *v != "space.cloudfoundry.crossplane.io" {
			t.Errorf("expected crossplane-kind=space.cloudfoundry.crossplane.io, got %v", v)
		}
		if v := m.Labels["crossplane-name"]; v == nil || *v != "my-space" {
			t.Errorf("expected crossplane-name=my-space, got %v", v)
		}
		if v := m.Labels["crossplane-providerconfig"]; v == nil || *v != "my-config" {
			t.Errorf("expected crossplane-providerconfig=my-config, got %v", v)
		}
		if len(m.Annotations) != 0 {
			t.Errorf("expected no annotations, got %d", len(m.Annotations))
		}
	})

	t.Run("defaults plus user labels", func(t *testing.T) {
		mg := newMockManaged("my-space", "my-config", spaceGVK)
		userLabels := map[string]*string{
			"env": ptrTo("production"),
		}
		m := BuildMetadata(mg, userLabels, nil)

		if len(m.Labels) != 4 {
			t.Fatalf("expected 4 labels (3 default + 1 user), got %d", len(m.Labels))
		}
		if v := m.Labels["env"]; v == nil || *v != "production" {
			t.Errorf("expected env=production, got %v", v)
		}
		// defaults still present
		if _, ok := m.Labels["crossplane-kind"]; !ok {
			t.Error("expected crossplane-kind label to be present")
		}
	})

	t.Run("user labels override defaults on collision", func(t *testing.T) {
		mg := newMockManaged("my-space", "my-config", spaceGVK)
		userLabels := map[string]*string{
			"crossplane-name": ptrTo("override-name"),
		}
		m := BuildMetadata(mg, userLabels, nil)

		if v := m.Labels["crossplane-name"]; v == nil || *v != "override-name" {
			t.Errorf("expected crossplane-name=override-name (user override), got %v", v)
		}
	})

	t.Run("no provider config ref - crossplane-providerconfig omitted", func(t *testing.T) {
		mg := newMockManaged("my-space", "", spaceGVK)
		m := BuildMetadata(mg, nil, nil)

		if len(m.Labels) != 2 {
			t.Fatalf("expected 2 labels (no providerconfig), got %d: %v", len(m.Labels), m.Labels)
		}
		if _, ok := m.Labels["crossplane-providerconfig"]; ok {
			t.Error("expected crossplane-providerconfig to be absent when no ProviderConfig ref")
		}
	})

	t.Run("defaults plus user labels and annotations", func(t *testing.T) {
		mg := newMockManaged("my-space", "my-config", spaceGVK)
		userLabels := map[string]*string{
			"env": ptrTo("staging"),
		}
		userAnnotations := map[string]*string{
			"description": ptrTo("my test space"),
		}
		m := BuildMetadata(mg, userLabels, userAnnotations)

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
		mg := newMockManaged("my-space", "my-config", spaceGVK)
		userLabels := map[string]*string{
			"stale-key": nil,
		}
		userAnnotations := map[string]*string{
			"stale-annotation": nil,
		}
		m := BuildMetadata(mg, userLabels, userAnnotations)

		// nil values should be present in the map as deletion markers
		if v, ok := m.Labels["stale-key"]; !ok {
			t.Error("expected stale-key to be present in labels (as deletion marker)")
		} else if v != nil {
			t.Errorf("expected nil deletion marker for stale-key, got %v", v)
		}
		if v, ok := m.Annotations["stale-annotation"]; !ok {
			t.Error("expected stale-annotation to be present in annotations (as deletion marker)")
		} else if v != nil {
			t.Errorf("expected nil deletion marker for stale-annotation, got %v", v)
		}
		// default labels still present
		if len(m.Labels) != 4 { // 3 default + 1 nil marker
			t.Errorf("expected 4 labels (3 default + 1 deletion marker), got %d: %v", len(m.Labels), m.Labels)
		}
	})

	t.Run("nil pointer value overrides default label", func(t *testing.T) {
		mg := newMockManaged("my-space", "my-config", spaceGVK)
		// User explicitly sets crossplane-name to nil (deletion marker)
		userLabels := map[string]*string{
			"crossplane-name": nil,
		}
		m := BuildMetadata(mg, userLabels, nil)

		if v, ok := m.Labels["crossplane-name"]; !ok {
			t.Error("expected crossplane-name to be present")
		} else if v != nil {
			t.Errorf("expected nil (deletion marker) for crossplane-name, got %v", v)
		}
	})
}

func TestMetadataMapEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		desired map[string]*string
		actual  map[string]*string
		want    bool
	}{
		{
			name:    "both nil",
			desired: nil,
			actual:  nil,
			want:    true,
		},
		{
			name:    "both empty",
			desired: map[string]*string{},
			actual:  map[string]*string{},
			want:    true,
		},
		{
			name:    "nil and empty",
			desired: nil,
			actual:  map[string]*string{},
			want:    true,
		},
		{
			name:    "same single key",
			desired: map[string]*string{"key": ptrTo("value")},
			actual:  map[string]*string{"key": ptrTo("value")},
			want:    true,
		},
		{
			name:    "different values",
			desired: map[string]*string{"key": ptrTo("a")},
			actual:  map[string]*string{"key": ptrTo("b")},
			want:    false,
		},
		{
			name:    "missing key in actual",
			desired: map[string]*string{"key": ptrTo("a"), "extra": ptrTo("b")},
			actual:  map[string]*string{"key": ptrTo("a")},
			want:    false,
		},
		{
			name:    "extra key in actual",
			desired: map[string]*string{"key": ptrTo("a")},
			actual:  map[string]*string{"key": ptrTo("a"), "extra": ptrTo("b")},
			want:    false,
		},
		{
			name:    "nil pointer vs nil pointer",
			desired: map[string]*string{"key": nil},
			actual:  map[string]*string{"key": nil},
			want:    true,
		},
		{
			name:    "nil pointer vs non-nil pointer",
			desired: map[string]*string{"key": nil},
			actual:  map[string]*string{"key": ptrTo("")},
			want:    false,
		},
		{
			name:    "non-nil pointer vs nil pointer",
			desired: map[string]*string{"key": ptrTo("val")},
			actual:  map[string]*string{"key": nil},
			want:    false,
		},
		{
			name:    "multiple matching keys",
			desired: map[string]*string{"a": ptrTo("1"), "b": ptrTo("2")},
			actual:  map[string]*string{"a": ptrTo("1"), "b": ptrTo("2")},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MetadataMapEqual(tt.desired, tt.actual)
			if got != tt.want {
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
		{
			name:    "both nil",
			desired: nil,
			actual:  nil,
			want:    true,
		},
		{
			name:    "desired nil actual has keys",
			desired: nil,
			actual:  map[string]*string{"key": ptrTo("value")},
			want:    true,
		},
		{
			name:    "desired empty actual has keys",
			desired: map[string]*string{},
			actual:  map[string]*string{"key": ptrTo("value")},
			want:    true,
		},
		{
			name:    "exact match",
			desired: map[string]*string{"key": ptrTo("value")},
			actual:  map[string]*string{"key": ptrTo("value")},
			want:    true,
		},
		{
			name:    "desired subset of actual",
			desired: map[string]*string{"key": ptrTo("value")},
			actual:  map[string]*string{"key": ptrTo("value"), "extra": ptrTo("data")},
			want:    true,
		},
		{
			name:    "desired key missing from actual",
			desired: map[string]*string{"key": ptrTo("value"), "missing": ptrTo("data")},
			actual:  map[string]*string{"key": ptrTo("value")},
			want:    false,
		},
		{
			name:    "desired value differs from actual",
			desired: map[string]*string{"key": ptrTo("new")},
			actual:  map[string]*string{"key": ptrTo("old"), "extra": ptrTo("data")},
			want:    false,
		},
		{
			name:    "nil pointer deletion marker match",
			desired: map[string]*string{"key": nil},
			actual:  map[string]*string{"key": nil, "extra": ptrTo("data")},
			want:    true,
		},
		{
			name:    "nil pointer vs non-nil pointer",
			desired: map[string]*string{"key": nil},
			actual:  map[string]*string{"key": ptrTo("")},
			want:    false,
		},
		{
			name:    "non-nil pointer vs nil pointer in actual",
			desired: map[string]*string{"key": ptrTo("val")},
			actual:  map[string]*string{"key": nil, "extra": ptrTo("data")},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MetadataMapContains(tt.desired, tt.actual)
			if got != tt.want {
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
		{
			name:               "all nil",
			desiredLabels:      nil,
			desiredAnnotations: nil,
			actualLabels:       nil,
			actualAnnotations:  nil,
			want:               true,
		},
		{
			name:               "labels match annotations nil",
			desiredLabels:      map[string]*string{"key": ptrTo("val")},
			desiredAnnotations: nil,
			actualLabels:       map[string]*string{"key": ptrTo("val")},
			actualAnnotations:  nil,
			want:               true,
		},
		{
			name:               "labels match annotations mismatch",
			desiredLabels:      map[string]*string{"key": ptrTo("val")},
			desiredAnnotations: map[string]*string{"note": ptrTo("a")},
			actualLabels:       map[string]*string{"key": ptrTo("val")},
			actualAnnotations:  map[string]*string{"note": ptrTo("b")},
			want:               false,
		},
		{
			name:               "labels mismatch",
			desiredLabels:      map[string]*string{"key": ptrTo("a")},
			desiredAnnotations: nil,
			actualLabels:       map[string]*string{"key": ptrTo("b")},
			actualAnnotations:  nil,
			want:               false,
		},
		{
			name:               "both match",
			desiredLabels:      map[string]*string{"key": ptrTo("val")},
			desiredAnnotations: map[string]*string{"note": ptrTo("a")},
			actualLabels:       map[string]*string{"key": ptrTo("val")},
			actualAnnotations:  map[string]*string{"note": ptrTo("a")},
			want:               true,
		},
		{
			name:               "actual has extra keys - still up to date",
			desiredLabels:      map[string]*string{"key": ptrTo("val")},
			desiredAnnotations: nil,
			actualLabels:       map[string]*string{"key": ptrTo("val"), "system-label": ptrTo("system-val")},
			actualAnnotations:  map[string]*string{"system-annotation": ptrTo("data")},
			want:               true,
		},
		{
			name:               "desired key missing from actual",
			desiredLabels:      map[string]*string{"key": ptrTo("val"), "missing": ptrTo("data")},
			desiredAnnotations: nil,
			actualLabels:       map[string]*string{"key": ptrTo("val")},
			actualAnnotations:  nil,
			want:               false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsMetadataUpToDate(tt.desiredLabels, tt.desiredAnnotations, tt.actualLabels, tt.actualAnnotations)
			if got != tt.want {
				t.Errorf("IsMetadataUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiffMetadata(t *testing.T) {
	t.Parallel()

	t.Run("no diff - identical maps", func(t *testing.T) {
		desired := map[string]*string{"a": ptrTo("1"), "b": ptrTo("2")}
		actual := map[string]*string{"a": ptrTo("1"), "b": ptrTo("2")}
		m := DiffMetadata(desired, nil, actual, nil)

		if len(m.Labels) != 0 {
			t.Errorf("expected empty label diff, got %d keys: %v", len(m.Labels), m.Labels)
		}
		if len(m.Annotations) != 0 {
			t.Errorf("expected empty annotation diff, got %d keys", len(m.Annotations))
		}
	})

	t.Run("add new key", func(t *testing.T) {
		desired := map[string]*string{"a": ptrTo("1"), "b": ptrTo("2")}
		actual := map[string]*string{"a": ptrTo("1")}
		m := DiffMetadata(desired, nil, actual, nil)

		if len(m.Labels) != 1 {
			t.Fatalf("expected 1 key in diff, got %d", len(m.Labels))
		}
		if v := m.Labels["b"]; v == nil || *v != "2" {
			t.Errorf("expected b=2 in diff, got %v", v)
		}
	})

	t.Run("update existing key", func(t *testing.T) {
		desired := map[string]*string{"a": ptrTo("new")}
		actual := map[string]*string{"a": ptrTo("old")}
		m := DiffMetadata(desired, nil, actual, nil)

		if len(m.Labels) != 1 {
			t.Fatalf("expected 1 key in diff, got %d", len(m.Labels))
		}
		if v := m.Labels["a"]; v == nil || *v != "new" {
			t.Errorf("expected a=new in diff, got %v", v)
		}
	})

	t.Run("keys in actual but not in desired are left alone", func(t *testing.T) {
		desired := map[string]*string{"a": ptrTo("1")}
		actual := map[string]*string{"a": ptrTo("1"), "system-label": ptrTo("system-val")}
		m := DiffMetadata(desired, nil, actual, nil)

		if len(m.Labels) != 0 {
			t.Errorf("expected empty diff (actual extra keys are ignored), got %d keys: %v", len(m.Labels), m.Labels)
		}
	})

	t.Run("explicit nil in desired produces deletion marker", func(t *testing.T) {
		desired := map[string]*string{"a": ptrTo("1"), "stale": nil}
		actual := map[string]*string{"a": ptrTo("1"), "stale": ptrTo("old-val")}
		m := DiffMetadata(desired, nil, actual, nil)

		if len(m.Labels) != 1 {
			t.Fatalf("expected 1 key in diff (deletion marker), got %d: %v", len(m.Labels), m.Labels)
		}
		if v, ok := m.Labels["stale"]; !ok {
			t.Error("expected stale key in diff")
		} else if v != nil {
			t.Errorf("expected nil deletion marker, got %v", v)
		}
	})

	t.Run("both nil maps", func(t *testing.T) {
		m := DiffMetadata(nil, nil, nil, nil)
		if len(m.Labels) != 0 {
			t.Errorf("expected empty label diff, got %d keys", len(m.Labels))
		}
		if len(m.Annotations) != 0 {
			t.Errorf("expected empty annotation diff, got %d keys", len(m.Annotations))
		}
	})

	t.Run("nil pointer value differs from non-nil", func(t *testing.T) {
		desired := map[string]*string{"a": nil}
		actual := map[string]*string{"a": ptrTo("value")}
		m := DiffMetadata(desired, nil, actual, nil)

		if len(m.Labels) != 1 {
			t.Fatalf("expected 1 key in diff, got %d", len(m.Labels))
		}
		if v := m.Labels["a"]; v != nil {
			t.Errorf("expected nil value in diff, got %v", v)
		}
	})

	t.Run("combined add and update", func(t *testing.T) {
		desired := map[string]*string{"a": ptrTo("1"), "c": ptrTo("3")}
		actual := map[string]*string{"a": ptrTo("old"), "b": ptrTo("2")}
		m := DiffMetadata(desired, nil, actual, nil)

		// a: updated, c: added, b: NOT in diff (left alone)
		if len(m.Labels) != 2 {
			t.Fatalf("expected 2 keys in diff, got %d: %v", len(m.Labels), m.Labels)
		}
		if v := m.Labels["a"]; v == nil || *v != "1" {
			t.Errorf("expected a=1 (update), got %v", v)
		}
		if v := m.Labels["c"]; v == nil || *v != "3" {
			t.Errorf("expected c=3 (add), got %v", v)
		}
		if _, ok := m.Labels["b"]; ok {
			t.Error("expected key 'b' to NOT be in diff (left alone)")
		}
	})
}

func TestDiffMetadata_Annotations(t *testing.T) {
	t.Parallel()

	t.Run("annotation diff only", func(t *testing.T) {
		desiredLabels := map[string]*string{"a": ptrTo("1")}
		desiredAnnotations := map[string]*string{"note": ptrTo("updated")}
		actualLabels := map[string]*string{"a": ptrTo("1")}
		actualAnnotations := map[string]*string{"note": ptrTo("old"), "extra": ptrTo("data")}
		m := DiffMetadata(desiredLabels, desiredAnnotations, actualLabels, actualAnnotations)

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
		desiredLabels := map[string]*string{"a": ptrTo("1"), "new": ptrTo("val")}
		desiredAnnotations := map[string]*string{"note": nil}
		actualLabels := map[string]*string{"a": ptrTo("old")}
		actualAnnotations := map[string]*string{"note": ptrTo("stale")}
		m := DiffMetadata(desiredLabels, desiredAnnotations, actualLabels, actualAnnotations)

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
		desiredLabels := map[string]*string{"a": ptrTo("1")}
		actualLabels := map[string]*string{"a": ptrTo("1")}
		actualAnnotations := map[string]*string{"system": ptrTo("data")}
		m := DiffMetadata(desiredLabels, nil, actualLabels, actualAnnotations)

		if len(m.Labels) != 0 {
			t.Errorf("expected empty label diff, got %d keys", len(m.Labels))
		}
		if len(m.Annotations) != 0 {
			t.Errorf("expected empty annotation diff (actual annotations ignored), got %d keys", len(m.Annotations))
		}
	})
}

func TestBuildMetadata_ProducesValidCFMetadata(t *testing.T) {
	spaceGVK := schema.GroupVersionKind{
		Group:   "cloudfoundry.crossplane.io",
		Version: "v1alpha1",
		Kind:    "Space",
	}
	mg := newMockManaged("test-space", "test-config", spaceGVK)
	userLabels := map[string]*string{"env": ptrTo("prod")}
	userAnnotations := map[string]*string{"note": ptrTo("test")}

	m := BuildMetadata(mg, userLabels, userAnnotations)

	if len(m.Labels) != 4 {
		t.Fatalf("expected 4 labels (3 default + 1 user), got %d", len(m.Labels))
	}
	if len(m.Annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(m.Annotations))
	}
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
		actual := map[string]*string{
			"env": ptrTo("prod"),
		}
		m := DiffMetadata(desired, nil, actual, nil)

		if len(m.Labels) != 3 {
			t.Fatalf("expected 3 keys to add (crossplane defaults), got %d: %v", len(m.Labels), m.Labels)
		}
		for _, k := range []string{"crossplane-kind", "crossplane-name", "crossplane-providerconfig"} {
			if _, ok := m.Labels[k]; !ok {
				t.Errorf("expected %q in diff", k)
			}
		}
	})

	t.Run("actual has stale defaults desired has updated values", func(t *testing.T) {
		desired := map[string]*string{
			"crossplane-name": ptrTo("new-space-name"),
		}
		actual := map[string]*string{
			"crossplane-name": ptrTo("old-space-name"),
		}
		m := DiffMetadata(desired, nil, actual, nil)

		if len(m.Labels) != 1 {
			t.Fatalf("expected 1 key in diff, got %d", len(m.Labels))
		}
		if v := m.Labels["crossplane-name"]; v == nil || *v != "new-space-name" {
			t.Errorf("expected crossplane-name=new-space-name, got %v", v)
		}
	})

	t.Run("actual has extra system labels - not in diff", func(t *testing.T) {
		desired := map[string]*string{
			"crossplane-kind": ptrTo("space.cloudfoundry.crossplane.io"),
			"crossplane-name": ptrTo("my-space"),
		}
		actual := map[string]*string{
			"crossplane-kind": ptrTo("space.cloudfoundry.crossplane.io"),
			"crossplane-name": ptrTo("my-space"),
			"cf-system-label": ptrTo("system-val"),
		}
		m := DiffMetadata(desired, nil, actual, nil)

		if len(m.Labels) != 0 {
			t.Errorf("expected empty diff (extra actual keys ignored), got %d keys: %v", len(m.Labels), m.Labels)
		}
	})
}
