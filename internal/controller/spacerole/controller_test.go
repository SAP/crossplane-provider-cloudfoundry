package spacerole

import (
	"context"
	"testing"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
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
	role "github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/role"
)

var (
	errBoom      = errors.New("boom")
	errSpace     = errors.New(role.ErrSpaceNotSpecified)
	resourceName = "my-space-roles"
	guidSpace    = "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a"
	guidRole     = "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69b"

	guidNoRefUser   = "2d8b0d04-d537-4e4e-8c6f-f09ca0e7f56f"
	guidHealthyUser = "1d1b0d04-d537-4e4e-8c6f-f09ca0e7f11f"

	healthyRole = &cfresource.Role{
		Resource: cfresource.Resource{
			GUID: guidRole},
		Type: "space_manager",
		Relationships: cfresource.RoleSpaceUserOrganizationRelationships{
			Space: cfresource.ToOneRelationship{
				Data: &cfresource.Relationship{
					GUID: guidSpace}},
			User: cfresource.ToOneRelationship{
				Data: &cfresource.Relationship{
					GUID: guidHealthyUser}}}}

	noSpaceRole = &cfresource.Role{
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

type modifier func(*v1alpha1.SpaceRole)

func withType(roleType string) modifier {
	return func(r *v1alpha1.SpaceRole) {
		r.Spec.ForProvider.Type = roleType
	}
}

func withUsername(username string) modifier {
	return func(r *v1alpha1.SpaceRole) {
		r.Spec.ForProvider.Username = username
	}
}

func withSpace(space string) modifier {
	return func(r *v1alpha1.SpaceRole) {
		r.Spec.ForProvider.Space = &space
	}
}

func withOrigin(origin string) modifier {
	return func(r *v1alpha1.SpaceRole) {
		r.Spec.ForProvider.Origin = &origin
	}
}

func withExternalName(name string) modifier {
	return func(r *v1alpha1.SpaceRole) {
		r.ObjectMeta.Annotations[meta.AnnotationKeyExternalName] = name
	}
}

func fakeSpaceRole(m ...modifier) *v1alpha1.SpaceRole {
	r := &v1alpha1.SpaceRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:        resourceName,
			Finalizers:  []string{},
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.SpaceRoleSpec{
			ForProvider: v1alpha1.SpaceRoleParameters{},
		},
	}

	for _, rm := range m {
		rm(r)
	}
	return r
}

func TestObserve(t *testing.T) {
	type service func() *fake.MockSpaceRole
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
			service: func() *fake.MockSpaceRole {
				m := &fake.MockSpaceRole{}
				return m
			},
		},
		"SpaceRelationNotSet": {
			args: args{
				mg: fakeSpaceRole(),
			},
			want: want{
				mg: fakeSpaceRole(),
				obs: managed.ExternalObservation{
					ResourceExists: false,
				},
				err: errors.Wrapf(errSpace, errGet),
			},
			service: func() *fake.MockSpaceRole {
				m := &fake.MockSpaceRole{}

				m.On("ListIncludeUsersAll").Return(
					[]*cfresource.Role{noSpaceRole},
					[]*cfresource.User{healthyUser},
					nil,
				)
				return m
			},
		},
		// This tests whether the external API is reachable
		"Boom!": {
			args: args{
				mg: fakeSpaceRole(withSpace("my-space"), withUsername("my-space-manager"), withType(v1alpha1.SpaceManager)),
			},
			want: want{
				mg:  fakeSpaceRole(withSpace("my-space"), withUsername("my-space-manager"), withType(v1alpha1.SpaceManager)),
				obs: managed.ExternalObservation{},
				err: errors.Wrap(errBoom, errGet),
			},
			service: func() *fake.MockSpaceRole {
				m := &fake.MockSpaceRole{}

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
		"NotFound by uuid is not found": {
			args: args{
				mg: fakeSpaceRole(withSpace("my-space"), withExternalName(guidRole)),
			},
			want: want{
				mg:  fakeSpaceRole(withSpace("my-space"), withExternalName(guidRole)),
				obs: managed.ExternalObservation{ResourceExists: false, ResourceUpToDate: false, ResourceLateInitialized: false},
				err: nil,
			},
			service: func() *fake.MockSpaceRole {
				m := &fake.MockSpaceRole{}

				m.On("Get", guidRole).Return(
					fake.SpaceRoleNil,
					nil,
				)
				return m
			},
		},
		"Successful": {
			args: args{
				mg: fakeSpaceRole(withSpace("my-space"), withUsername("user1"), withType(v1alpha1.SpaceManager)),
			},
			want: want{
				mg:  fakeSpaceRole(withSpace("my-space"), withUsername("user1"), withType(v1alpha1.SpaceManager), withExternalName(guidRole)),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ResourceLateInitialized: true},
				err: nil,
			},
			service: func() *fake.MockSpaceRole {
				m := &fake.MockSpaceRole{}

				m.On("ListIncludeUsersAll").Return(
					[]*cfresource.Role{healthyRole},
					[]*cfresource.User{healthyUser},
					nil,
				)
				return m
			},
		},
		"Successful when SpaceRole guid is found": {
			args: args{
				mg: fakeSpaceRole(withSpace("my-space"), withUsername("user1"), withType(v1alpha1.SpaceManager), withExternalName(guidRole)),
			},
			want: want{
				mg:  fakeSpaceRole(withSpace("my-space"), withUsername("user1"), withType(v1alpha1.SpaceManager), withExternalName(guidRole)),
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			service: func() *fake.MockSpaceRole {
				m := &fake.MockSpaceRole{}

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
	type service func() *fake.MockSpaceRole
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
		"Successful": {
			args: args{
				mg: fakeSpaceRole(
					withType(v1alpha1.SpaceManager),
					withUsername("user1@test.com"),
					withSpace("my-space"),
					withOrigin("my-origin"),
				),
			},
			want: want{
				mg: fakeSpaceRole(
					withType(v1alpha1.SpaceManager),
					withUsername("user1@test.com"),
					withSpace("my-space"),
					withOrigin("my-origin"),
					withExternalName(guidSpace),
				),
				obs: managed.ExternalCreation{ConnectionDetails: managed.ConnectionDetails{}},
				err: nil,
			},
			service: func() *fake.MockSpaceRole {
				m := &fake.MockSpaceRole{}
				m.On("CreateSpaceRoleWithUsername").Return(
					&fake.NewSpaceRole().SetType("organization_manager").SetGUID(guidSpace).SetRelationships(cfresource.RoleSpaceUserOrganizationRelationships{User: cfresource.ToOneRelationship{Data: &cfresource.Relationship{GUID: guidHealthyUser}}, Space: cfresource.ToOneRelationship{Data: &cfresource.Relationship{GUID: guidSpace}}}).Role,
					nil,
				)

				return m
			},
		},
		"NoType": {
			args: args{
				mg: fakeSpaceRole(
					withUsername("my-space-manager"),
					withSpace("my-space"),
					withOrigin("my-origin"),
				),
			},
			want: want{
				mg: fakeSpaceRole(
					withUsername("my-space-manager"),
					withSpace("my-space"),
					withOrigin("my-origin"),
				),
				obs: managed.ExternalCreation{},
				err: errors.New(errCreate),
			},
			service: func() *fake.MockSpaceRole {
				m := &fake.MockSpaceRole{}

				var emptyRole *cfresource.Role
				m.On("CreateSpaceRoleWithUsername").Return(
					emptyRole,
					errCreate,
				)
				return m
			},
		},
		"NoOrg": {
			args: args{
				mg: fakeSpaceRole(
					withType(v1alpha1.SpaceManager),
					withUsername("my-space-manager"),
					withOrigin("my-origin"),
				),
			},
			want: want{
				mg: fakeSpaceRole(
					withType(v1alpha1.SpaceManager),
					withUsername("my-space-manager"),
					withOrigin("my-origin"),
				),
				obs: managed.ExternalCreation{},
				err: errors.New(errCreate),
			},
			service: func() *fake.MockSpaceRole {
				m := &fake.MockSpaceRole{}

				var emptyRole *cfresource.Role
				m.On("CreateSpaceRoleWithUsername").Return(
					emptyRole,
					errCreate,
				)
				return m
			},
		},
		"NoUsername": {
			args: args{
				mg: fakeSpaceRole(
					withType(v1alpha1.SpaceManager),
					withSpace("my-space"),
					withOrigin("my-origin"),
				),
			},
			want: want{
				mg: fakeSpaceRole(
					withType(v1alpha1.SpaceManager),
					withSpace("my-space"),
					withOrigin("my-origin"),
				),
				obs: managed.ExternalCreation{},
				err: errors.New(errCreate),
			},
			service: func() *fake.MockSpaceRole {
				m := &fake.MockSpaceRole{}

				var emptyRole *cfresource.Role
				m.On("CreateSpaceRoleWithUsername").Return(
					emptyRole,
					errCreate,
				)
				return m
			},
		},

		"AlreadyExist": {
			args: args{
				mg: fakeSpaceRole(
					withType(v1alpha1.SpaceManager),
					withUsername("my-space-manager"),
					withSpace("my-space"),
					withOrigin("my-origin"),
				),
			},
			want: want{
				mg: fakeSpaceRole(
					withType(v1alpha1.SpaceManager),
					withUsername("my-space-manager"),
					withSpace("my-space"),
					withOrigin("my-origin"),
				),
				obs: managed.ExternalCreation{},
				err: errors.Wrap(errBoom, errCreate),
			},
			service: func() *fake.MockSpaceRole {
				m := &fake.MockSpaceRole{}

				var emptyRole *cfresource.Role
				m.On("CreateSpaceRoleWithUsername").Return(
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
