package space

import (
	"testing"

	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/go-cmp/cmp"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

func TestGenerateCreate(t *testing.T) {
	type args struct {
		spec v1alpha1.SpaceParameters
	}
	type want struct {
		create *resource.SpaceCreate
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"BasicCreate": {
			args: args{
				spec: v1alpha1.SpaceParameters{
					Name: "test-space",
					OrgReference: v1alpha1.OrgReference{
						Org: ptr.To("test-org-guid"),
					},
				},
			},
			want: want{
				create: func() *resource.SpaceCreate {
					c := resource.NewSpaceCreate("test-space", "test-org-guid")
					c.Metadata = &resource.Metadata{}
					return c
				}(),
			},
		},
		"CreateWithLabels": {
			args: args{
				spec: v1alpha1.SpaceParameters{
					Name: "test-space",
					OrgReference: v1alpha1.OrgReference{
						Org: ptr.To("test-org-guid"),
					},
					ResourceMetadata: v1alpha1.ResourceMetadata{
						Labels: map[string]*string{
							"env": ptr.To("prod"),
						},
					},
				},
			},
			want: want{
				create: func() *resource.SpaceCreate {
					c := resource.NewSpaceCreate("test-space", "test-org-guid")
					c.Metadata = &resource.Metadata{
						Labels: map[string]*string{
							"env": ptr.To("prod"),
						},
					}
					return c
				}(),
			},
		},
		"CreateWithLabelsAndAnnotations": {
			args: args{
				spec: v1alpha1.SpaceParameters{
					Name: "test-space",
					OrgReference: v1alpha1.OrgReference{
						Org: ptr.To("test-org-guid"),
					},
					ResourceMetadata: v1alpha1.ResourceMetadata{
						Labels: map[string]*string{
							"env": ptr.To("staging"),
						},
						Annotations: map[string]*string{
							"note": ptr.To("test-annotation"),
						},
					},
				},
			},
			want: want{
				create: func() *resource.SpaceCreate {
					c := resource.NewSpaceCreate("test-space", "test-org-guid")
					c.Metadata = &resource.Metadata{
						Labels: map[string]*string{
							"env": ptr.To("staging"),
						},
						Annotations: map[string]*string{
							"note": ptr.To("test-annotation"),
						},
					}
					return c
				}(),
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			result := GenerateCreate(nil, tc.args.spec)
			if diff := cmp.Diff(tc.want.create, result); diff != "" {
				t.Errorf("GenerateCreate(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestGenerateUpdate(t *testing.T) {
	type args struct {
		spec v1alpha1.SpaceParameters
	}
	type want struct {
		update *resource.SpaceUpdate
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"BasicUpdate": {
			args: args{
				spec: v1alpha1.SpaceParameters{
					Name: "test-space",
				},
			},
			want: want{
				update: &resource.SpaceUpdate{
					Name:     "test-space",
					Metadata: &resource.Metadata{},
				},
			},
		},
		"UpdateWithLabels": {
			args: args{
				spec: v1alpha1.SpaceParameters{
					Name: "test-space",
					ResourceMetadata: v1alpha1.ResourceMetadata{
						Labels: map[string]*string{
							"env": ptr.To("prod"),
						},
					},
				},
			},
			want: want{
				update: &resource.SpaceUpdate{
					Name: "test-space",
					Metadata: &resource.Metadata{
						Labels: map[string]*string{
							"env": ptr.To("prod"),
						},
					},
				},
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			result := GenerateUpdate(nil, tc.args.spec)
			if diff := cmp.Diff(tc.want.update, result); diff != "" {
				t.Errorf("GenerateUpdate(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestGenerateObservation(t *testing.T) {
	cases := map[string]struct {
		space      *resource.Space
		ssh        bool
		wantID     string
		wantName   string
		wantOrg    string
		wantSSH    bool
		wantLabels map[string]*string
		wantAnns   map[string]*string
	}{
		"WithMetadata": {
			space: &resource.Space{
				Name: "test-space",
				Relationships: &resource.SpaceRelationships{
					Organization: &resource.ToOneRelationship{Data: &resource.Relationship{GUID: "org-guid"}},
				},
				Metadata: &resource.Metadata{
					Labels:      map[string]*string{"env": ptr.To("prod")},
					Annotations: map[string]*string{"note": ptr.To("test")},
				},
				Resource: resource.Resource{GUID: "space-guid"},
			},
			ssh:        true,
			wantID:     "space-guid",
			wantName:   "test-space",
			wantOrg:    "org-guid",
			wantSSH:    true,
			wantLabels: map[string]*string{"env": ptr.To("prod")},
			wantAnns:   map[string]*string{"note": ptr.To("test")},
		},
		"NilMetadata": {
			space: &resource.Space{
				Name: "test-space",
				Relationships: &resource.SpaceRelationships{
					Organization: &resource.ToOneRelationship{Data: &resource.Relationship{GUID: "org-guid"}},
				},
				Resource: resource.Resource{GUID: "space-guid"},
			},
			ssh:      false,
			wantID:   "space-guid",
			wantName: "test-space",
			wantOrg:  "org-guid",
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			result := GenerateObservation(tc.space, tc.ssh)
			if result.ID != tc.wantID {
				t.Errorf("ID: want %q, got %q", tc.wantID, result.ID)
			}
			if result.Name != tc.wantName {
				t.Errorf("Name: want %q, got %q", tc.wantName, result.Name)
			}
			if result.Org != tc.wantOrg {
				t.Errorf("Org: want %q, got %q", tc.wantOrg, result.Org)
			}
			if result.AllowSSH != tc.wantSSH {
				t.Errorf("AllowSSH: want %v, got %v", tc.wantSSH, result.AllowSSH)
			}
			if diff := cmp.Diff(tc.wantLabels, result.Labels); diff != "" {
				t.Errorf("Labels: -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantAnns, result.Annotations); diff != "" {
				t.Errorf("Annotations: -want, +got:\n%s", diff)
			}
		})
	}
}

func TestIsUpToDate(t *testing.T) {
	cases := map[string]struct {
		spec     v1alpha1.SpaceParameters
		observed *resource.Space
		ssh      bool
		want     bool
	}{
		"UpToDateNoLabels": {
			spec:     v1alpha1.SpaceParameters{Name: "test-space"},
			observed: &resource.Space{Name: "test-space"},
			ssh:      false,
			want:     true,
		},
		"NameDrift": {
			spec:     v1alpha1.SpaceParameters{Name: "new-name"},
			observed: &resource.Space{Name: "old-name"},
			ssh:      false,
			want:     false,
		},
		"SSHDrift": {
			spec:     v1alpha1.SpaceParameters{Name: "test-space", AllowSSH: true},
			observed: &resource.Space{Name: "test-space"},
			ssh:      false,
			want:     false,
		},
		"LabelDrift": {
			spec: v1alpha1.SpaceParameters{
				Name: "test-space",
				ResourceMetadata: v1alpha1.ResourceMetadata{
					Labels: map[string]*string{"env": ptr.To("prod")},
				},
			},
			observed: &resource.Space{Name: "test-space"},
			ssh:      false,
			want:     false,
		},
		"LabelsMatch": {
			spec: v1alpha1.SpaceParameters{
				Name: "test-space",
				ResourceMetadata: v1alpha1.ResourceMetadata{
					Labels: map[string]*string{"env": ptr.To("prod")},
				},
			},
			observed: &resource.Space{
				Name:     "test-space",
				Metadata: &resource.Metadata{Labels: map[string]*string{"env": ptr.To("prod")}},
			},
			ssh:  false,
			want: true,
		},
		"AnnotationDrift": {
			spec: v1alpha1.SpaceParameters{
				Name: "test-space",
				ResourceMetadata: v1alpha1.ResourceMetadata{
					Annotations: map[string]*string{"note": ptr.To("value")},
				},
			},
			observed: &resource.Space{Name: "test-space"},
			ssh:      false,
			want:     false,
		},
		"AnnotationsMatch": {
			spec: v1alpha1.SpaceParameters{
				Name: "test-space",
				ResourceMetadata: v1alpha1.ResourceMetadata{
					Annotations: map[string]*string{"note": ptr.To("value")},
				},
			},
			observed: &resource.Space{
				Name:     "test-space",
				Metadata: &resource.Metadata{Annotations: map[string]*string{"note": ptr.To("value")}},
			},
			ssh:  false,
			want: true,
		},
		"ObservedNilMetadata": {
			spec:     v1alpha1.SpaceParameters{Name: "test-space"},
			observed: &resource.Space{Name: "test-space"},
			ssh:      false,
			want:     true,
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			result := IsUpToDate(nil, tc.spec, tc.observed, tc.ssh)
			if result != tc.want {
				t.Errorf("IsUpToDate(...): want %v, got %v", tc.want, result)
			}
		})
	}
}
