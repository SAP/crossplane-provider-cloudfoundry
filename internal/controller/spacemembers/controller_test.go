package spacemembers

import (
	"context"
	"fmt"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

var (
	errBoom      = errors.New("boom")
	spaceGUID    = "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a"
	roleType     = "Manager"
	extName      = composeExternalName(spaceGUID, roleType)
	resourceName = "my-space-members"

	assignedRoles = map[string]string{
		"user1|sap.ids": "role-guid-1",
		"user2|sap.ids": "role-guid-2",
	}
)

type modifier func(*v1alpha1.SpaceMembers)

func withExternalName(name string) modifier {
	return func(r *v1alpha1.SpaceMembers) {
		r.Annotations[meta.AnnotationKeyExternalName] = name
	}
}

func withSpace(space string) modifier {
	return func(r *v1alpha1.SpaceMembers) {
		r.Spec.ForProvider.Space = &space
	}
}

func withRoleType(roleType string) modifier {
	return func(r *v1alpha1.SpaceMembers) {
		r.Spec.ForProvider.RoleType = roleType
	}
}

func withAssignedRoles(roles map[string]string) modifier {
	return func(r *v1alpha1.SpaceMembers) {
		r.Status.AtProvider.AssignedRoles = roles
	}
}

func fakeSpaceMembers(m ...modifier) *v1alpha1.SpaceMembers {
	r := &v1alpha1.SpaceMembers{
		ObjectMeta: metav1.ObjectMeta{
			Name:        resourceName,
			Finalizers:  []string{},
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.SpaceMembersSpec{
			ForProvider: v1alpha1.SpaceMembersParameters{
				RoleType: roleType,
			},
		},
	}

	for _, mod := range m {
		mod(r)
	}
	return r
}

// mockMembersClient is a mock for the members.Client methods used by the SpaceMembers controller.
type mockMembersClient struct {
	observeFn func(ctx context.Context, cr *v1alpha1.SpaceMembers) (*v1alpha1.RoleAssignments, error)
	assignFn  func(ctx context.Context, cr *v1alpha1.SpaceMembers) (*v1alpha1.RoleAssignments, error)
	updateFn  func(ctx context.Context, cr *v1alpha1.SpaceMembers) (*v1alpha1.RoleAssignments, error)
	deleteFn  func(ctx context.Context, cr *v1alpha1.SpaceMembers) error
}

func (m *mockMembersClient) ObserveSpaceMembers(ctx context.Context, cr *v1alpha1.SpaceMembers) (*v1alpha1.RoleAssignments, error) {
	return m.observeFn(ctx, cr)
}

func (m *mockMembersClient) AssignSpaceMembers(ctx context.Context, cr *v1alpha1.SpaceMembers) (*v1alpha1.RoleAssignments, error) {
	return m.assignFn(ctx, cr)
}

func (m *mockMembersClient) UpdateSpaceMembers(ctx context.Context, cr *v1alpha1.SpaceMembers) (*v1alpha1.RoleAssignments, error) {
	return m.updateFn(ctx, cr)
}

func (m *mockMembersClient) DeleteSpaceMembers(ctx context.Context, cr *v1alpha1.SpaceMembers) error {
	return m.deleteFn(ctx, cr)
}

func TestObserve(t *testing.T) {
	type want struct {
		obs managed.ExternalObservation
		err error
	}

	cases := map[string]struct {
		cr   *v1alpha1.SpaceMembers
		want want
		mock mockMembersClient
	}{
		"SpaceNotResolved": {
			cr:   fakeSpaceMembers(withRoleType(roleType)),
			want: want{obs: managed.ExternalObservation{}, err: errors.New(errSpaceNotResolved)},
			mock: mockMembersClient{},
		},
		// ADR Step 1: Empty external-name should be late-initialized from spec
		"EmptyExternalNameLateInitialized": {
			cr: fakeSpaceMembers(withSpace(spaceGUID), withRoleType(roleType)),
			want: want{
				obs: managed.ExternalObservation{
					ResourceExists:          false,
					ResourceLateInitialized: true,
				},
				err: nil,
			},
			mock: mockMembersClient{},
		},
		// External-name set to metadata.Name (Crossplane default) should be treated as empty
		"ExternalNameIsMetadataName": {
			cr: fakeSpaceMembers(withSpace(spaceGUID), withRoleType(roleType), withExternalName(resourceName)),
			want: want{
				obs: managed.ExternalObservation{
					ResourceExists:          false,
					ResourceLateInitialized: true,
				},
				err: nil,
			},
			mock: mockMembersClient{},
		},
		// Legacy format (just a space GUID) should be migrated to compound key
		"LegacyExternalNameMigrated": {
			cr: fakeSpaceMembers(withSpace(spaceGUID), withRoleType(roleType), withExternalName(spaceGUID)),
			want: want{
				obs: managed.ExternalObservation{
					ResourceExists:          false,
					ResourceLateInitialized: true,
				},
				err: nil,
			},
			mock: mockMembersClient{},
		},
		// Invalid format (no slash) should return error
		"InvalidFormatNoSlash": {
			cr: fakeSpaceMembers(withSpace(spaceGUID), withRoleType(roleType), withExternalName("invalid-no-slash")),
			want: want{
				obs: managed.ExternalObservation{},
				err: fmt.Errorf(errExternalNameFmt, "invalid-no-slash"),
			},
			mock: mockMembersClient{},
		},
		// Invalid GUID portion in compound key
		"InvalidGUIDInCompoundKey": {
			cr: fakeSpaceMembers(withSpace(spaceGUID), withRoleType(roleType), withExternalName("not-a-guid/Manager")),
			want: want{
				obs: managed.ExternalObservation{},
				err: errors.New("space GUID 'not-a-guid' in external-name is not a valid UUID format"),
			},
			mock: mockMembersClient{},
		},
		// Valid compound key, resource observed successfully
		"SuccessfulObserve": {
			cr: fakeSpaceMembers(withSpace(spaceGUID), withRoleType(roleType), withExternalName(extName)),
			want: want{
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			mock: mockMembersClient{
				observeFn: func(ctx context.Context, cr *v1alpha1.SpaceMembers) (*v1alpha1.RoleAssignments, error) {
					return &v1alpha1.RoleAssignments{AssignedRoles: assignedRoles}, nil
				},
			},
		},
		// Valid compound key, observed state not consistent with CR (needs update)
		"ResourceNotUpToDate": {
			cr: fakeSpaceMembers(withSpace(spaceGUID), withRoleType(roleType), withExternalName(extName)),
			want: want{
				obs: managed.ExternalObservation{ResourceExists: false, ResourceUpToDate: false},
				err: nil,
			},
			mock: mockMembersClient{
				observeFn: func(ctx context.Context, cr *v1alpha1.SpaceMembers) (*v1alpha1.RoleAssignments, error) {
					return nil, nil
				},
			},
		},
		// Valid compound key with existing assigned roles, observed state not up to date
		"ResourceExistsWithStateUpdateNeeded": {
			cr: fakeSpaceMembers(withSpace(spaceGUID), withRoleType(roleType), withExternalName(extName), withAssignedRoles(assignedRoles)),
			want: want{
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false},
				err: nil,
			},
			mock: mockMembersClient{
				observeFn: func(ctx context.Context, cr *v1alpha1.SpaceMembers) (*v1alpha1.RoleAssignments, error) {
					return nil, nil
				},
			},
		},
		// Read error from API
		"ObserveError": {
			cr: fakeSpaceMembers(withSpace(spaceGUID), withRoleType(roleType), withExternalName(extName)),
			want: want{
				obs: managed.ExternalObservation{},
				err: errors.Wrap(errBoom, errRead),
			},
			mock: mockMembersClient{
				observeFn: func(ctx context.Context, cr *v1alpha1.SpaceMembers) (*v1alpha1.RoleAssignments, error) {
					return nil, errBoom
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &external{client: &tc.mock}
			obs, err := c.Observe(context.Background(), tc.cr)

			if tc.want.err != nil {
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
			if tc.cr != nil {
				if name == "EmptyExternalNameLateInitialized" || name == "ExternalNameIsMetadataName" || name == "LegacyExternalNameMigrated" {
					gotExtName := meta.GetExternalName(tc.cr)
					if gotExtName != extName {
						t.Errorf("Observe(...): external-name want %q, got %q", extName, gotExtName)
					}
				}
			}
		})
	}
}

func TestCreate(t *testing.T) {
	type want struct {
		extName string
		err     error
	}

	cases := map[string]struct {
		cr   *v1alpha1.SpaceMembers
		want want
		mock mockMembersClient
	}{
		"SuccessfulCreate": {
			cr:   fakeSpaceMembers(withSpace(spaceGUID), withRoleType(roleType)),
			want: want{extName: extName, err: nil},
			mock: mockMembersClient{
				assignFn: func(ctx context.Context, cr *v1alpha1.SpaceMembers) (*v1alpha1.RoleAssignments, error) {
					return &v1alpha1.RoleAssignments{AssignedRoles: assignedRoles}, nil
				},
			},
		},
		"CreateError": {
			cr:   fakeSpaceMembers(withSpace(spaceGUID), withRoleType(roleType)),
			want: want{extName: "", err: errors.Wrap(errBoom, errCreate)},
			mock: mockMembersClient{
				assignFn: func(ctx context.Context, cr *v1alpha1.SpaceMembers) (*v1alpha1.RoleAssignments, error) {
					return nil, errBoom
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &external{client: &tc.mock}
			_, err := c.Create(context.Background(), tc.cr)

			if tc.want.err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Create(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("Create(...): want error != got error:\n%s", diff)
				}
				if tc.want.extName != "" && tc.cr != nil {
					gotExtName := meta.GetExternalName(tc.cr)
					if diff := cmp.Diff(tc.want.extName, gotExtName); diff != "" {
						t.Errorf("Create(...): external-name -want, +got:\n%s", diff)
					}
				}
			}
		})
	}
}

func TestDelete(t *testing.T) {
	type want struct {
		err error
	}

	cases := map[string]struct {
		cr   *v1alpha1.SpaceMembers
		want want
		mock mockMembersClient
	}{
		"SuccessfulDelete": {
			cr:   fakeSpaceMembers(withSpace(spaceGUID), withRoleType(roleType), withExternalName(extName), withAssignedRoles(assignedRoles)),
			want: want{err: nil},
			mock: mockMembersClient{
				deleteFn: func(ctx context.Context, cr *v1alpha1.SpaceMembers) error {
					return nil
				},
			},
		},
		"NothingToDeleteWhenNoAssignedRoles": {
			cr:   fakeSpaceMembers(withSpace(spaceGUID), withRoleType(roleType), withExternalName(extName)),
			want: want{err: nil},
			mock: mockMembersClient{},
		},
		"DeleteError": {
			cr:   fakeSpaceMembers(withSpace(spaceGUID), withRoleType(roleType), withExternalName(extName), withAssignedRoles(assignedRoles)),
			want: want{err: errors.Wrap(errBoom, errDelete)},
			mock: mockMembersClient{
				deleteFn: func(ctx context.Context, cr *v1alpha1.SpaceMembers) error {
					return errBoom
				},
			},
		},
		"DeleteNotFound": {
			cr:   fakeSpaceMembers(withSpace(spaceGUID), withRoleType(roleType), withExternalName(extName), withAssignedRoles(assignedRoles)),
			want: want{err: nil},
			mock: mockMembersClient{
				deleteFn: func(ctx context.Context, cr *v1alpha1.SpaceMembers) error {
					return fmt.Errorf("NotFound: resource not found")
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &external{client: &tc.mock}
			_, err := c.Delete(context.Background(), tc.cr)

			if tc.want.err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Delete(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("Delete(...): want error != got error:\n%s", diff)
				}
			}
		})
	}
}

func TestParseExternalName(t *testing.T) {
	cases := map[string]struct {
		input     string
		wantSpace string
		wantRole  string
		wantErr   bool
	}{
		"ValidCompoundKey": {
			input:     "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a/Manager",
			wantSpace: "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a",
			wantRole:  "Manager",
			wantErr:   false,
		},
		"ValidCompoundKeyWithDeveloper": {
			input:     "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a/Developer",
			wantSpace: "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a",
			wantRole:  "Developer",
			wantErr:   false,
		},
		"EmptyString": {
			input:   "",
			wantErr: true,
		},
		"NoSlashJustGUID": {
			input:   "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a",
			wantErr: true,
		},
		"TooManySlashes": {
			input:   "guid/part/extra",
			wantErr: true,
		},
		"EmptySpaceGUID": {
			input:   "/Manager",
			wantErr: true,
		},
		"EmptyRoleType": {
			input:   "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a/",
			wantErr: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			spaceGUID, roleType, err := parseExternalName(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseExternalName(%q): expected error, got none", tc.input)
				}
			} else {
				if err != nil {
					t.Errorf("parseExternalName(%q): unexpected error: %v", tc.input, err)
				}
				if diff := cmp.Diff(tc.wantSpace, spaceGUID); diff != "" {
					t.Errorf("parseExternalName(%q): spaceGUID -want, +got:\n%s", tc.input, diff)
				}
				if diff := cmp.Diff(tc.wantRole, roleType); diff != "" {
					t.Errorf("parseExternalName(%q): roleType -want, +got:\n%s", tc.input, diff)
				}
			}
		})
	}
}

func TestComposeExternalName(t *testing.T) {
	result := composeExternalName("9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a", "Manager")
	expected := "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a/Manager"
	if diff := cmp.Diff(expected, result); diff != "" {
		t.Errorf("composeExternalName(): -want, +got:\n%s", diff)
	}
}

func TestIsOldExternalNameFormat(t *testing.T) {
	cases := map[string]struct {
		input    string
		expected bool
	}{
		"ValidGUID":   {"9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a", true},
		"CompoundKey": {"9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a/Manager", false},
		"EmptyString": {"", false},
		"RandomName":  {"my-resource", false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := isOldExternalNameFormat(tc.input)
			if diff := cmp.Diff(tc.expected, result); diff != "" {
				t.Errorf("isOldExternalNameFormat(%q): -want, +got:\n%s", tc.input, diff)
			}
		})
	}
}
