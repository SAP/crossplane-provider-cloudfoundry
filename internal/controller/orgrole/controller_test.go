package orgrole

import (
	"context"
	"testing"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/role"
)

var (
	errBoom            = errors.New("boom")
	errOrgNotSpecified = errors.New(role.ErrOrgNotSpecified)
	resourceName       = "my-org-roles"
	guidOrg            = "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a"
	guidRole           = "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69b"

	guidNoRefUser   = "2d8b0d04-d537-4e4e-8c6f-f09ca0e7f56f"
	guidHealthyUser = "1d1b0d04-d537-4e4e-8c6f-f09ca0e7f11f"

	healthyRole = &cfresource.Role{
		Resource: cfresource.Resource{
			GUID: guidRole},
		Type: "organization_manager",
		Relationships: cfresource.RoleSpaceUserOrganizationRelationships{
			Org: cfresource.ToOneRelationship{
				Data: &cfresource.Relationship{
					GUID: guidOrg}},
			User: cfresource.ToOneRelationship{
				Data: &cfresource.Relationship{
					GUID: guidHealthyUser}}}}

	noOrgRole = &cfresource.Role{
		Relationships: cfresource.RoleSpaceUserOrganizationRelationships{
			User: cfresource.ToOneRelationship{
				Data: &cfresource.Relationship{
					GUID: guidNoRefUser}}}}

	healthyUser = &cfresource.User{
		Username: ptr.To("user1"),
		Origin:   ptr.To("sap.ids"),
		Resource: cfresource.Resource{
			GUID: guidHealthyUser}}
)

type modifier func(*v1alpha1.OrgRole)

func withType(roleType string) modifier {
	return func(r *v1alpha1.OrgRole) {
		r.Spec.ForProvider.Type = roleType
	}
}

func withUsername(username string) modifier {
	return func(r *v1alpha1.OrgRole) {
		r.Spec.ForProvider.Username = username
	}
}

func withOrg(org string) modifier {
	return func(r *v1alpha1.OrgRole) {
		r.Spec.ForProvider.Org = &org
	}
}

func withOrgName(orgName string) modifier {
	return func(r *v1alpha1.OrgRole) {
		r.Spec.ForProvider.OrgName = &orgName
	}
}

func withOrigin(origin string) modifier {
	return func(r *v1alpha1.OrgRole) {
		r.Spec.ForProvider.Origin = &origin
	}
}

func withExternalName(name string) modifier {
	return func(r *v1alpha1.OrgRole) {
		r.ObjectMeta.Annotations[meta.AnnotationKeyExternalName] = name
	}
}

func withConditions(c ...xpv1.Condition) modifier {
	return func(i *v1alpha1.OrgRole) { i.Status.SetConditions(c...) }
}

func fakeOrgRole(m ...modifier) *v1alpha1.OrgRole {
	r := &v1alpha1.OrgRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:        resourceName,
			Finalizers:  []string{},
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.OrgRoleSpec{
			ForProvider: v1alpha1.OrgRoleParameters{},
		},
	}

	for _, rm := range m {
		rm(r)
	}
	return r
}

func TestObserve(t *testing.T) {
	type service func() *fake.MockOrgRole
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  resource.Managed
		obs managed.ExternalObservation
		err error
	}

	cases := map[string]struct {
		args    args
		want    want
		service service
		kube    k8s.Client
	}{
		"Nil": {
			args: args{
				mg: nil,
			},
			want: want{
				obs: managed.ExternalObservation{},
				err: errors.New(errWrongKind),
			},
			service: func() *fake.MockOrgRole {
				m := &fake.MockOrgRole{}
				return m
			},
		},
		"OrgRelationNotSet": {
			args: args{
				mg: fakeOrgRole(),
			},
			want: want{
				mg: fakeOrgRole(),
				obs: managed.ExternalObservation{
					ResourceExists: false,
				},
				err: errors.Wrapf(errOrgNotSpecified, errGet),
			},
			service: func() *fake.MockOrgRole {
				m := &fake.MockOrgRole{}

				m.On("ListIncludeUsersAll").Return(
					[]*cfresource.Role{noOrgRole},
					[]*cfresource.User{healthyUser},
					nil,
				)
				return m
			},
		},
		// This tests whether the external API is reachable
		"Boom!": {
			args: args{
				mg: fakeOrgRole(withOrg(guidOrg)),
			},
			want: want{
				mg:  fakeOrgRole(withOrg(guidOrg)),
				obs: managed.ExternalObservation{},
				err: errors.Wrap(errBoom, errGet),
			},
			service: func() *fake.MockOrgRole {
				m := &fake.MockOrgRole{}

				var emptyRole []*cfresource.Role
				var emptyUser []*cfresource.User

				m.On("ListIncludeUsersAll").Return(
					emptyRole,
					emptyUser,
					errBoom,
				)
				return m
			},
		},
		"NotFoundByUUID": {
			args: args{
				mg: fakeOrgRole(
					withOrg(guidOrg),
					withUsername("user1"),
					withType(v1alpha1.OrgManager),
					withExternalName(guidRole)),
			},
			want: want{
				mg: fakeOrgRole(
					withOrg(guidOrg),
					withUsername("user1"),
					withType(v1alpha1.OrgManager),
					withExternalName(guidRole)),
				obs: managed.ExternalObservation{ResourceExists: false, ResourceUpToDate: false, ResourceLateInitialized: false},
				err: nil,
			},
			service: func() *fake.MockOrgRole {
				m := &fake.MockOrgRole{}

				m.On("Get", guidRole).Return(
					fake.OrganizationRoleNil,
					nil,
				)
				return m
			},
		},
		"Successful": {
			args: args{
				mg: fakeOrgRole(
					withOrg(guidOrg),
					withUsername("user1"),
					withType(v1alpha1.OrgManager)),
			},
			want: want{
				mg: fakeOrgRole(
					withOrg(guidOrg),
					withUsername("user1"),
					withType(v1alpha1.OrgManager),
					withExternalName(guidRole)),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ResourceLateInitialized: true},
				err: nil,
			},
			service: func() *fake.MockOrgRole {
				m := &fake.MockOrgRole{}

				m.On("ListIncludeUsersAll").Return(
					[]*cfresource.Role{healthyRole},
					[]*cfresource.User{healthyUser},
					nil,
				)
				return m
			},
		},
		"SuccessfulWithUUID": {
			args: args{
				mg: fakeOrgRole(
					withOrg(guidOrg),
					withExternalName(guidRole)),
			},
			want: want{
				mg: fakeOrgRole(
					withOrg(guidOrg),
					withUsername("user1"),
					withType(v1alpha1.OrgManager),
					withExternalName(guidRole)),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ResourceLateInitialized: false},
				err: nil,
			},
			service: func() *fake.MockOrgRole {
				m := &fake.MockOrgRole{}

				m.On("Get", guidRole).Return(
					healthyRole,
					nil,
				)
				return m
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			c := &external{
				kube: &test.MockClient{
					MockUpdate: test.NewMockUpdateFn(nil),
				},
				job:  nil,
				role: tc.service(),
			}
			obs, err := c.Observe(context.Background(), tc.args.mg)

			if tc.want.err != nil && err != nil {
				// the case where our mock server returns error.
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Observe(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("Observe(...): want error != got error:\n%s", diff)
				}
			}
			if diff := cmp.Diff(tc.want.obs, obs); diff != "" {
				t.Errorf("Observe(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	type service func() *fake.MockOrgRole
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  resource.Managed
		obs managed.ExternalCreation
		err error
	}

	cases := map[string]struct {
		args    args
		want    want
		service service
		kube    k8s.Client
	}{
		"SuccessfulWithOrgGUID": {
			args: args{
				mg: fakeOrgRole(
					withType(v1alpha1.OrgManager),
					withUsername("user1@test.com"),
					withOrg("my-org"),
					withOrigin("sap.ids"),
				),
			},
			want: want{
				mg: fakeOrgRole(
					withType(v1alpha1.OrgManager),
					withUsername("user1@test.com"),
					withOrg("my-org"),
					withOrigin("my-origin"),
					withExternalName(guidOrg),
				),
				obs: managed.ExternalCreation{ConnectionDetails: managed.ConnectionDetails{}},
				err: nil,
			},
			service: func() *fake.MockOrgRole {
				m := &fake.MockOrgRole{}
				m.On("CreateOrganizationRoleWithUsername").Return(
					&fake.NewOrgRole().SetType("organization_manager").SetGUID(guidOrg).SetRelationships(cfresource.RoleSpaceUserOrganizationRelationships{User: cfresource.ToOneRelationship{Data: &cfresource.Relationship{GUID: guidHealthyUser}}, Org: cfresource.ToOneRelationship{Data: &cfresource.Relationship{GUID: guidOrg}}}).Role,
					nil,
				)

				return m
			},
		},
		"SuccessfulWithOrgName": {
			args: args{
				mg: fakeOrgRole(
					withType(v1alpha1.OrgManager),
					withUsername("user1@test.com"),
					withOrgName("my-org"),
					withOrg("my-org"),
					withOrigin("sap.ids"),
				),
			},
			want: want{
				mg: fakeOrgRole(
					withType(v1alpha1.OrgManager),
					withUsername("user1@test.com"),
					withOrgName("my-org"),
					withOrg("my-org"),
					withOrigin("my-origin"),
					withExternalName(guidOrg),
				),
				obs: managed.ExternalCreation{ConnectionDetails: managed.ConnectionDetails{}},
				err: nil,
			},
			service: func() *fake.MockOrgRole {
				m := &fake.MockOrgRole{}
				m.On("CreateOrganizationRoleWithUsername").Return(
					&fake.NewOrgRole().SetType("organization_manager").SetGUID(guidOrg).SetRelationships(cfresource.RoleSpaceUserOrganizationRelationships{User: cfresource.ToOneRelationship{Data: &cfresource.Relationship{GUID: guidHealthyUser}}, Org: cfresource.ToOneRelationship{Data: &cfresource.Relationship{GUID: guidOrg}}}).Role,
					nil,
				)

				return m
			},
		},
		"NoType": {
			args: args{
				mg: fakeOrgRole(
					withUsername("my-org-manager"),
					withOrg("my-org"),
					withOrigin("my-origin"),
				),
			},
			want: want{
				mg: fakeOrgRole(
					withUsername("my-org-manager"),
					withOrg("my-org"),
					withOrigin("my-origin"),
				),
				obs: managed.ExternalCreation{},
				err: errors.New(errCreate),
			},
			service: func() *fake.MockOrgRole {
				m := &fake.MockOrgRole{}

				var emptyRole *cfresource.Role
				m.On("CreateOrganizationRoleWithUsername").Return(
					emptyRole,
					errCreate,
				)
				return m
			},
		},
		"NoOrg": {
			args: args{
				mg: fakeOrgRole(
					withType(v1alpha1.OrgManager),
					withUsername("my-org-manager"),
					withOrigin("my-origin"),
				),
			},
			want: want{
				mg: fakeOrgRole(
					withType(v1alpha1.OrgManager),
					withUsername("my-org-manager"),
					withOrigin("my-origin"),
				),
				obs: managed.ExternalCreation{},
				err: errors.New(errCreate),
			},
			service: func() *fake.MockOrgRole {
				m := &fake.MockOrgRole{}

				var emptyRole *cfresource.Role
				m.On("CreateOrganizationRoleWithUsername").Return(
					emptyRole,
					errCreate,
				)
				return m
			},
		},
		"NoUsername": {
			args: args{
				mg: fakeOrgRole(
					withType(v1alpha1.OrgManager),
					withOrg("my-org"),
					withOrigin("my-origin"),
				),
			},
			want: want{
				mg: fakeOrgRole(
					withType(v1alpha1.OrgManager),
					withOrg("my-org"),
					withOrigin("my-origin"),
				),
				obs: managed.ExternalCreation{},
				err: errors.New(errCreate),
			},
			service: func() *fake.MockOrgRole {
				m := &fake.MockOrgRole{}

				var emptyRole *cfresource.Role
				m.On("CreateOrganizationRoleWithUsername").Return(
					emptyRole,
					errCreate,
				)
				return m
			},
		},
		"AlreadyExist": {
			args: args{
				mg: fakeOrgRole(
					withType(v1alpha1.OrgManager),
					withUsername("my-org-manager"),
					withOrg("my-org"),
					withOrigin("my-origin"),
				),
			},
			want: want{
				mg: fakeOrgRole(
					withType(v1alpha1.OrgManager),
					withUsername("my-org-manager"),
					withOrg("my-org"),
					withOrigin("my-origin"),
				),
				obs: managed.ExternalCreation{},
				err: errors.Wrap(errBoom, errCreate),
			},
			service: func() *fake.MockOrgRole {
				m := &fake.MockOrgRole{}

				var emptyRole *cfresource.Role
				m.On("CreateOrganizationRoleWithUsername").Return(
					emptyRole,
					errBoom,
				)

				return m
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			t.Logf("Testing: %s", t.Name())
			c := &external{
				kube: &test.MockClient{
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				job:  nil,
				role: tc.service(),
			}

			obs, err := c.Create(context.Background(), tc.args.mg)

			if tc.want.err != nil && err != nil {
				// the case where our mock server returns error.
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Observe(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("Observe(...): want error != got error:\n%s", diff)
				}
			}
			if diff := cmp.Diff(tc.want.obs, obs); diff != "" {
				t.Errorf("Observe(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	type service func() *fake.MockOrgRole
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  resource.Managed
		obs managed.ExternalUpdate
		err error
	}

	cases := map[string]struct {
		args    args
		want    want
		service service
		kube    k8s.Client
	}{
		"SuccessfulDelete": {
			args: args{
				mg: fakeOrgRole(
					withType(v1alpha1.OrgManager),
					withUsername("user1@test.com"),
					withOrg("my-org"),
					withOrigin("sap.ids"),
				),
			},
			want: want{
				mg: fakeOrgRole(
					withType(v1alpha1.OrgManager),
					withUsername("user1@test.com"),
					withOrg("my-org"),
					withOrigin("sap.ids"),
					withConditions(xpv1.Deleting()),
				),
				obs: managed.ExternalUpdate{},
				err: nil,
			},
			service: func() *fake.MockOrgRole {
				m := &fake.MockOrgRole{}

				m.On("Delete").Return(
					"",
					nil,
				)

				return m
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			t.Logf("Testing: %s", t.Name())
			c := &external{
				kube: &test.MockClient{
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				job:  nil,
				role: tc.service(),
			}

			_, err := c.Delete(context.Background(), tc.args.mg)

			if tc.want.err != nil && err != nil {
				// the case where our mock server returns error.
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Observe(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("Observe(...): want error != got error:\n%s", diff)
				}
			}
			if diff := cmp.Diff(tc.want.mg, tc.args.mg); diff != "" {
				t.Errorf("Observe(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestInitialize(t *testing.T) {
	type service func() *fake.MockOrgRole
	type args struct {
		mg resource.Managed
	}

	type want struct {
		mg  resource.Managed
		err error
	}

	cases := map[string]struct {
		args    args
		want    want
		service service
	}{
		"Nil": {
			args: args{
				mg: nil,
			},
			want: want{
				err: errors.New(errWrongKind),
			},
			service: func() *fake.MockOrgRole {
				m := &fake.MockOrgRole{}
				return m
			},
		},
		"SuccessfulWithOrgGUID": {
			args: args{
				mg: fakeOrgRole(
					withType(v1alpha1.OrgManager),
					withUsername("my-org-manager"),
					withOrg("my-org"),
					withOrigin("my-origin"),
				),
			},
			want: want{
				mg: fakeOrgRole(
					withType(v1alpha1.OrgManager),
					withUsername("my-org-manager"),
					withOrg("my-org"),
					withOrigin("my-origin"),
				),
				err: nil,
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			c := &orgInitializer{
				kube: &test.MockClient{
					MockUpdate:       test.NewMockUpdateFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
			}
			err := c.Initialize(context.Background(), tc.args.mg)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Initialize(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("Initialize(...): want error != got error:\n%s", diff)
				}
			}
		})
	}
}
