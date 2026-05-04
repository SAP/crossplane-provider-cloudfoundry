package domain

import (
	"testing"

	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/go-cmp/cmp"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

func TestGenerateCreate(t *testing.T) {
	type args struct {
		spec v1alpha1.DomainParameters
	}
	type want struct {
		create *resource.DomainCreate
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"BasicCreate": {
			args: args{
				spec: v1alpha1.DomainParameters{
					Name: "test.domain.com",
				},
			},
			want: want{
				create: &resource.DomainCreate{
					Name:     "test.domain.com",
					Metadata: &resource.Metadata{},
				},
			},
		},
		"CreateWithLabels": {
			args: args{
				spec: v1alpha1.DomainParameters{
					Name: "test.domain.com",
					ResourceMetadata: v1alpha1.ResourceMetadata{
						Labels: map[string]*string{
							"env": ptr.To("prod"),
						},
					},
				},
			},
			want: want{
				create: &resource.DomainCreate{
					Name: "test.domain.com",
					Metadata: &resource.Metadata{
						Labels: map[string]*string{
							"env": ptr.To("prod"),
						},
					},
				},
			},
		},
		"CreateInternalFalseWithRouterGroup": {
			args: args{
				spec: v1alpha1.DomainParameters{
					Name:        "test.domain.com",
					Internal:    ptr.To(false),
					RouterGroup: ptr.To("rg-guid"),
				},
			},
			want: want{
				create: &resource.DomainCreate{
					Name:        "test.domain.com",
					Internal:    ptr.To(false),
					RouterGroup: &resource.Relationship{GUID: "rg-guid"},
					Metadata:    &resource.Metadata{},
				},
			},
		},
		"CreateWithOrg": {
			args: args{
				spec: v1alpha1.DomainParameters{
					Name: "test.domain.com",
					OrgReference: v1alpha1.OrgReference{
						Org: ptr.To("org-guid"),
					},
				},
			},
			want: want{
				create: &resource.DomainCreate{
					Name: "test.domain.com",
					Relationships: &resource.DomainRelationships{
						Organization: &resource.ToOneRelationship{
							Data: &resource.Relationship{GUID: "org-guid"},
						},
					},
					Metadata: &resource.Metadata{},
				},
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
		spec v1alpha1.DomainParameters
	}
	type want struct {
		update *resource.DomainUpdate
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"BasicUpdate": {
			args: args{
				spec: v1alpha1.DomainParameters{
					Name: "test.domain.com",
				},
			},
			want: want{
				update: &resource.DomainUpdate{
					Metadata: &resource.Metadata{},
				},
			},
		},
		"UpdateWithLabels": {
			args: args{
				spec: v1alpha1.DomainParameters{
					Name: "test.domain.com",
					ResourceMetadata: v1alpha1.ResourceMetadata{
						Labels: map[string]*string{
							"env": ptr.To("prod"),
						},
					},
				},
			},
			want: want{
				update: &resource.DomainUpdate{
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
		domain     *resource.Domain
		wantID     string
		wantName   string
		wantLabels map[string]*string
		wantAnns   map[string]*string
	}{
		"WithMetadata": {
			domain: &resource.Domain{
				Name: "test.domain.com",
				Metadata: &resource.Metadata{
					Labels:      map[string]*string{"env": ptr.To("prod")},
					Annotations: map[string]*string{"note": ptr.To("test")},
				},
				Resource: resource.Resource{GUID: "domain-guid"},
			},
			wantID:     "domain-guid",
			wantName:   "test.domain.com",
			wantLabels: map[string]*string{"env": ptr.To("prod")},
			wantAnns:   map[string]*string{"note": ptr.To("test")},
		},
		"NilMetadata": {
			domain: &resource.Domain{
				Name:     "test.domain.com",
				Resource: resource.Resource{GUID: "domain-guid"},
			},
			wantID:   "domain-guid",
			wantName: "test.domain.com",
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			result := GenerateObservation(tc.domain)
			if result.ID == nil || *result.ID != tc.wantID {
				t.Errorf("ID: want %q, got %v", tc.wantID, result.ID)
			}
			if result.Name == nil || *result.Name != tc.wantName {
				t.Errorf("Name: want %q, got %v", tc.wantName, result.Name)
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
		spec     v1alpha1.DomainParameters
		observed *resource.Domain
		want     bool
	}{
		"NilObserved": {
			spec:     v1alpha1.DomainParameters{Name: "test.domain.com"},
			observed: nil,
			want:     false,
		},
		"UpToDateNoLabels": {
			spec:     v1alpha1.DomainParameters{Name: "test.domain.com"},
			observed: &resource.Domain{Name: "test.domain.com"},
			want:     true,
		},
		"LabelDrift": {
			spec: v1alpha1.DomainParameters{
				Name: "test.domain.com",
				ResourceMetadata: v1alpha1.ResourceMetadata{
					Labels: map[string]*string{"env": ptr.To("prod")},
				},
			},
			observed: &resource.Domain{Name: "test.domain.com"},
			want:     false,
		},
		"LabelsMatch": {
			spec: v1alpha1.DomainParameters{
				Name: "test.domain.com",
				ResourceMetadata: v1alpha1.ResourceMetadata{
					Labels: map[string]*string{"env": ptr.To("prod")},
				},
			},
			observed: &resource.Domain{
				Name:     "test.domain.com",
				Metadata: &resource.Metadata{Labels: map[string]*string{"env": ptr.To("prod")}},
			},
			want: true,
		},
		"AnnotationDrift": {
			spec: v1alpha1.DomainParameters{
				Name: "test.domain.com",
				ResourceMetadata: v1alpha1.ResourceMetadata{
					Annotations: map[string]*string{"note": ptr.To("value")},
				},
			},
			observed: &resource.Domain{Name: "test.domain.com"},
			want:     false,
		},
		"AnnotationsMatch": {
			spec: v1alpha1.DomainParameters{
				Name: "test.domain.com",
				ResourceMetadata: v1alpha1.ResourceMetadata{
					Annotations: map[string]*string{"note": ptr.To("value")},
				},
			},
			observed: &resource.Domain{
				Name:     "test.domain.com",
				Metadata: &resource.Metadata{Annotations: map[string]*string{"note": ptr.To("value")}},
			},
			want: true,
		},
		"ObservedNilMetadata": {
			spec:     v1alpha1.DomainParameters{Name: "test.domain.com"},
			observed: &resource.Domain{Name: "test.domain.com"},
			want:     true,
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			result := IsUpToDate(nil, tc.spec, tc.observed)
			if result != tc.want {
				t.Errorf("IsUpToDate(...): want %v, got %v", tc.want, result)
			}
		})
	}
}
