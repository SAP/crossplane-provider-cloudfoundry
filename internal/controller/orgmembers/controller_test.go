package orgmembers

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
	errBoom  = errors.New("boom")
	orgGUID  = "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a"
	roleType = "Manager"
	extName  = composeExternalName(orgGUID, roleType)
	resName  = "my-org-members"

	assignedRoles = map[string]string{
		"user1|sap.ids": "role-guid-1",
		"user2|sap.ids": "role-guid-2",
	}
)

type modifier func(*v1alpha1.OrgMembers)

func withExternalName(name string) modifier {
	return func(r *v1alpha1.OrgMembers) {
		r.Annotations[meta.AnnotationKeyExternalName] = name
	}
}

func withOrg(org string) modifier {
	return func(r *v1alpha1.OrgMembers) {
		r.Spec.ForProvider.Org = &org
	}
}

func withRoleType(rt string) modifier {
	return func(r *v1alpha1.OrgMembers) {
		r.Spec.ForProvider.RoleType = rt
	}
}

func withAssignedRoles(roles map[string]string) modifier {
	return func(r *v1alpha1.OrgMembers) {
		r.Status.AtProvider.AssignedRoles = roles
	}
}

func fakeOrgMembers(m ...modifier) *v1alpha1.OrgMembers {
	r := &v1alpha1.OrgMembers{
		ObjectMeta: metav1.ObjectMeta{
			Name:        resName,
			Finalizers:  []string{},
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.OrgMembersSpec{
			ForProvider: v1alpha1.OrgMembersParameters{
				RoleType: roleType,
			},
		},
	}

	for _, mod := range m {
		mod(r)
	}
	return r
}

// mockOrgMemberClient is a mock for the orgMemberClient interface.
type mockOrgMemberClient struct {
	observeFn func(ctx context.Context, orgGUID, roleType string, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, bool, error)
	assignFn  func(ctx context.Context, orgGUID, roleType string, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, error)
	updateFn  func(ctx context.Context, orgGUID, roleType string, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, error)
	deleteFn  func(ctx context.Context, orgGUID, roleType string, cr *v1alpha1.OrgMembers) error
}

func (m *mockOrgMemberClient) ObserveOrgMembers(ctx context.Context, orgGUID, roleType string, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, bool, error) {
	if m.observeFn == nil {
		return nil, false, nil
	}
	return m.observeFn(ctx, orgGUID, roleType, cr)
}

func (m *mockOrgMemberClient) AssignOrgMembers(ctx context.Context, orgGUID, roleType string, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, error) {
	if m.assignFn == nil {
		return nil, nil
	}
	return m.assignFn(ctx, orgGUID, roleType, cr)
}

func (m *mockOrgMemberClient) UpdateOrgMembers(ctx context.Context, orgGUID, roleType string, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, error) {
	if m.updateFn == nil {
		return nil, nil
	}
	return m.updateFn(ctx, orgGUID, roleType, cr)
}

func (m *mockOrgMemberClient) DeleteOrgMembers(ctx context.Context, orgGUID, roleType string, cr *v1alpha1.OrgMembers) error {
	if m.deleteFn == nil {
		return nil
	}
	return m.deleteFn(ctx, orgGUID, roleType, cr)
}

func TestObserve(t *testing.T) {
	type want struct {
		obs managed.ExternalObservation
		err error
	}

	cases := map[string]struct {
		cr   *v1alpha1.OrgMembers
		want want
		mock mockOrgMemberClient
	}{
		"OrgNotResolved": {
			cr:   fakeOrgMembers(withRoleType(roleType)),
			want: want{obs: managed.ExternalObservation{}, err: errors.New(errOrgNotResolved)},
			mock: mockOrgMemberClient{},
		},
		"RoleTypeRequiredWithoutExternalName": {
			cr:   fakeOrgMembers(withOrg(orgGUID), withRoleType("")),
			want: want{obs: managed.ExternalObservation{}, err: errors.New(errRoleTypeRequired)},
			mock: mockOrgMemberClient{},
		},
		// ADR Step 1: Empty external-name should be late-initialized from spec
		"EmptyExternalNameLateInitialized": {
			cr: fakeOrgMembers(withOrg(orgGUID), withRoleType(roleType)),
			want: want{
				obs: managed.ExternalObservation{
					ResourceExists:          false,
					ResourceLateInitialized: true,
				},
				err: nil,
			},
			mock: mockOrgMemberClient{},
		},
		// External-name set to metadata.Name (Crossplane default) should be treated as empty
		"ExternalNameIsMetadataName": {
			cr: fakeOrgMembers(withOrg(orgGUID), withRoleType(roleType), withExternalName(resName)),
			want: want{
				obs: managed.ExternalObservation{
					ResourceExists:          false,
					ResourceLateInitialized: true,
				},
				err: nil,
			},
			mock: mockOrgMemberClient{},
		},
		// Legacy format (RoleType@OrgGUID) should be migrated to compound key
		"LegacyExternalNameMigrated": {
			cr: fakeOrgMembers(withOrg(orgGUID), withRoleType(roleType), withExternalName("Manager@9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a")),
			want: want{
				obs: managed.ExternalObservation{
					ResourceExists:          false,
					ResourceLateInitialized: true,
				},
				err: nil,
			},
			mock: mockOrgMemberClient{},
		},
		// Invalid format (no slash, no @) should return error
		"InvalidFormatNoSlashNoAt": {
			cr: fakeOrgMembers(withOrg(orgGUID), withRoleType(roleType), withExternalName("invalid-no-slash")),
			want: want{
				obs: managed.ExternalObservation{},
				err: fmt.Errorf(errExternalNameFmt, "invalid-no-slash"),
			},
			mock: mockOrgMemberClient{},
		},
		// Invalid GUID portion in compound key
		"InvalidGUIDInCompoundKey": {
			cr: fakeOrgMembers(withOrg(orgGUID), withRoleType(roleType), withExternalName("not-a-guid/Manager")),
			want: want{
				obs: managed.ExternalObservation{},
				err: errors.New("org GUID 'not-a-guid' in external-name is not a valid UUID format"),
			},
			mock: mockOrgMemberClient{},
		},
		// Valid compound key, resource observed successfully
		"SuccessfulObserve": {
			cr: fakeOrgMembers(withOrg(orgGUID), withRoleType(roleType), withExternalName(extName)),
			want: want{
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			mock: mockOrgMemberClient{
				observeFn: func(ctx context.Context, gotOrgGUID, gotRoleType string, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, bool, error) {
					if gotOrgGUID != orgGUID || gotRoleType != roleType {
						return nil, false, fmt.Errorf("unexpected identity: %s/%s", gotOrgGUID, gotRoleType)
					}
					return &v1alpha1.RoleAssignments{AssignedRoles: assignedRoles}, true, nil
				},
			},
		},
		// Valid compound key, observed state not consistent with CR (needs update)
		"ResourceNotUpToDate": {
			cr: fakeOrgMembers(withOrg(orgGUID), withRoleType(roleType), withExternalName(extName)),
			want: want{
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false},
				err: nil,
			},
			mock: mockOrgMemberClient{
				observeFn: func(ctx context.Context, _, _ string, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, bool, error) {
					return nil, true, nil
				},
			},
		},
		// Valid compound key with existing assigned roles, observed state not up to date
		"ResourceExistsWithStateUpdateNeeded": {
			cr: fakeOrgMembers(withOrg(orgGUID), withRoleType(roleType), withExternalName(extName), withAssignedRoles(assignedRoles)),
			want: want{
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false},
				err: nil,
			},
			mock: mockOrgMemberClient{
				observeFn: func(ctx context.Context, _, _ string, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, bool, error) {
					return nil, true, nil
				},
			},
		},
		"IdentityConflictOrgGUID": {
			cr: fakeOrgMembers(withOrg(orgGUID), withRoleType(roleType), withExternalName(composeExternalName("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", roleType))),
			want: want{
				obs: managed.ExternalObservation{},
				err: errors.New("identity conflict: external-name org (aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee) differs from spec (9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a)"),
			},
			mock: mockOrgMemberClient{},
		},
		"IdentityConflictRoleType": {
			cr: fakeOrgMembers(withOrg(orgGUID), withRoleType("Auditor"), withExternalName(extName)),
			want: want{
				obs: managed.ExternalObservation{},
				err: errors.New("identity conflict: external-name role type (Manager) differs from spec (Auditor)"),
			},
			mock: mockOrgMemberClient{},
		},
		"NoConflictWhenRoleTypeEmptyForObserveOnly": {
			cr: fakeOrgMembers(withOrg(orgGUID), withRoleType(""), withExternalName(extName)),
			want: want{
				obs: managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true},
				err: nil,
			},
			mock: mockOrgMemberClient{
				observeFn: func(ctx context.Context, gotOrgGUID, gotRoleType string, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, bool, error) {
					if gotOrgGUID != orgGUID || gotRoleType != roleType {
						return nil, false, fmt.Errorf("unexpected identity: %s/%s", gotOrgGUID, gotRoleType)
					}
					return &v1alpha1.RoleAssignments{AssignedRoles: assignedRoles}, true, nil
				},
			},
		},
		// Read error from API
		"ObserveError": {
			cr: fakeOrgMembers(withOrg(orgGUID), withRoleType(roleType), withExternalName(extName)),
			want: want{
				obs: managed.ExternalObservation{},
				err: errors.Wrap(errBoom, errRead),
			},
			mock: mockOrgMemberClient{
				observeFn: func(ctx context.Context, _, _ string, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, bool, error) {
					return nil, false, errBoom
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
		cr   *v1alpha1.OrgMembers
		want want
		mock mockOrgMemberClient
	}{
		"SuccessfulCreate": {
			cr:   fakeOrgMembers(withOrg(orgGUID), withRoleType(roleType)),
			want: want{extName: extName, err: nil},
			mock: mockOrgMemberClient{
				assignFn: func(ctx context.Context, gotOrgGUID, gotRoleType string, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, error) {
					if gotOrgGUID != orgGUID || gotRoleType != roleType {
						return nil, fmt.Errorf("unexpected identity: %s/%s", gotOrgGUID, gotRoleType)
					}
					return &v1alpha1.RoleAssignments{AssignedRoles: assignedRoles}, nil
				},
			},
		},
		"CreateError": {
			cr:   fakeOrgMembers(withOrg(orgGUID), withRoleType(roleType)),
			want: want{extName: "", err: errors.Wrap(errBoom, errCreate)},
			mock: mockOrgMemberClient{
				assignFn: func(ctx context.Context, _, _ string, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, error) {
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

func TestUpdate(t *testing.T) {
	type want struct {
		extName string
		err     error
	}

	cases := map[string]struct {
		cr   *v1alpha1.OrgMembers
		want want
		mock mockOrgMemberClient
	}{
		"SuccessfulUpdate": {
			cr:   fakeOrgMembers(withOrg(orgGUID), withRoleType(roleType), withExternalName(extName)),
			want: want{extName: extName, err: nil},
			mock: mockOrgMemberClient{
				updateFn: func(ctx context.Context, gotOrgGUID, gotRoleType string, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, error) {
					if gotOrgGUID != orgGUID || gotRoleType != roleType {
						return nil, fmt.Errorf("unexpected identity: %s/%s", gotOrgGUID, gotRoleType)
					}
					return &v1alpha1.RoleAssignments{AssignedRoles: assignedRoles}, nil
				},
			},
		},
		"UpdateError": {
			cr:   fakeOrgMembers(withOrg(orgGUID), withRoleType(roleType), withExternalName(extName)),
			want: want{extName: "", err: errors.Wrap(errBoom, errUpdate)},
			mock: mockOrgMemberClient{
				updateFn: func(ctx context.Context, _, _ string, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, error) {
					return nil, errBoom
				},
			},
		},
		// Update should not rewrite the external-name
		"UpdateDoesNotRewriteExternalName": {
			cr:   fakeOrgMembers(withOrg(orgGUID), withRoleType(roleType), withExternalName(extName)),
			want: want{extName: extName, err: nil},
			mock: mockOrgMemberClient{
				updateFn: func(ctx context.Context, _, _ string, cr *v1alpha1.OrgMembers) (*v1alpha1.RoleAssignments, error) {
					return &v1alpha1.RoleAssignments{AssignedRoles: assignedRoles}, nil
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &external{client: &tc.mock}
			_, err := c.Update(context.Background(), tc.cr)

			if tc.want.err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("Update(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("Update(...): want error != got error:\n%s", diff)
				}
				if tc.want.extName != "" && tc.cr != nil {
					gotExtName := meta.GetExternalName(tc.cr)
					if diff := cmp.Diff(tc.want.extName, gotExtName); diff != "" {
						t.Errorf("Update(...): external-name -want, +got:\n%s", diff)
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
		cr   *v1alpha1.OrgMembers
		want want
		mock mockOrgMemberClient
	}{
		"SuccessfulDelete": {
			cr:   fakeOrgMembers(withOrg(orgGUID), withRoleType(roleType), withExternalName(extName), withAssignedRoles(assignedRoles)),
			want: want{err: nil},
			mock: mockOrgMemberClient{
				deleteFn: func(ctx context.Context, gotOrgGUID, gotRoleType string, cr *v1alpha1.OrgMembers) error {
					if gotOrgGUID != orgGUID || gotRoleType != roleType {
						return fmt.Errorf("unexpected identity: %s/%s", gotOrgGUID, gotRoleType)
					}
					return nil
				},
			},
		},
		"DeleteEmptyRolesStillCallsClient": {
			cr:   fakeOrgMembers(withOrg(orgGUID), withRoleType(roleType), withExternalName(extName)),
			want: want{err: nil},
			mock: mockOrgMemberClient{
				deleteFn: func(ctx context.Context, gotOrgGUID, gotRoleType string, cr *v1alpha1.OrgMembers) error {
					if gotOrgGUID != orgGUID || gotRoleType != roleType {
						return fmt.Errorf("unexpected identity: %s/%s", gotOrgGUID, gotRoleType)
					}
					return nil
				},
			},
		},
		"DeleteError": {
			cr:   fakeOrgMembers(withOrg(orgGUID), withRoleType(roleType), withExternalName(extName), withAssignedRoles(assignedRoles)),
			want: want{err: errors.Wrap(errBoom, errDelete)},
			mock: mockOrgMemberClient{
				deleteFn: func(ctx context.Context, _, _ string, cr *v1alpha1.OrgMembers) error {
					return errBoom
				},
			},
		},
		"DeleteNotFound": {
			cr:   fakeOrgMembers(withOrg(orgGUID), withRoleType(roleType), withExternalName(extName), withAssignedRoles(assignedRoles)),
			want: want{err: nil},
			mock: mockOrgMemberClient{
				deleteFn: func(ctx context.Context, _, _ string, cr *v1alpha1.OrgMembers) error {
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
		input   string
		wantOrg string
		wantRt  string
		wantErr bool
	}{
		"ValidCompoundKey": {
			input:   "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a/Manager",
			wantOrg: "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a",
			wantRt:  "Manager",
			wantErr: false,
		},
		"ValidCompoundKeyWithUser": {
			input:   "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a/User",
			wantOrg: "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a",
			wantRt:  "User",
			wantErr: false,
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
		"EmptyOrgGUID": {
			input:   "/Manager",
			wantErr: true,
		},
		"EmptyRoleType": {
			input:   "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a/",
			wantErr: true,
		},
		"PluralManagersCanonicalized": {
			input:   "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a/Managers",
			wantOrg: "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a",
			wantRt:  "Manager",
			wantErr: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotOrg, gotRt, err := parseExternalName(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseExternalName(%q): expected error, got none", tc.input)
				}
			} else {
				if err != nil {
					t.Errorf("parseExternalName(%q): unexpected error: %v", tc.input, err)
				}
				if diff := cmp.Diff(tc.wantOrg, gotOrg); diff != "" {
					t.Errorf("parseExternalName(%q): orgGUID -want, +got:\n%s", tc.input, diff)
				}
				if diff := cmp.Diff(tc.wantRt, gotRt); diff != "" {
					t.Errorf("parseExternalName(%q): roleType -want, +got:\n%s", tc.input, diff)
				}
			}
		})
	}
}

func TestComposeExternalName(t *testing.T) {
	cases := map[string]struct {
		orgGUID  string
		roleType string
		want      string
	}{
		"SingularManager": {"9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a", "Manager", "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a/Manager"},
		"PluralManagersCanonicalized": {"9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a", "Managers", "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a/Manager"},
		"PluralUsersCanonicalized":    {"9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a", "Users", "9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a/User"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := composeExternalName(tc.orgGUID, tc.roleType)
			if diff := cmp.Diff(tc.want, result); diff != "" {
				t.Errorf("composeExternalName(%q, %q): -want, +got:\n%s", tc.orgGUID, tc.roleType, diff)
			}
		})
	}
}

func TestIsOldExternalNameFormat(t *testing.T) {
	cases := map[string]struct {
		input    string
		expected bool
	}{
		"LegacyRoleTypeAtOrgGUID": {"Manager@9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a", true},
		"LegacyUserAtOrgGUID":     {"User@9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a", true},
		"CompoundKey":             {"9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a/Manager", false},
		"EmptyString":             {"", false},
		"RandomName":              {"my-resource", false},
		"JustGUID":                {"9e4b0d04-d537-6a6a-8c6f-f09ca0e7f69a", false},
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

func TestCanonicalizeRoleType(t *testing.T) {
	cases := map[string]struct {
		input string
		want  string
	}{
		"SingularManager":      {"Manager", "Manager"},
		"PluralManagers":       {"Managers", "Manager"},
		"SingularUser":         {"User", "User"},
		"PluralUsers":          {"Users", "User"},
		"SingularAuditor":      {"Auditor", "Auditor"},
		"PluralAuditors":       {"Auditors", "Auditor"},
		"SingularBillingManager": {"BillingManager", "BillingManager"},
		"PluralBillingManagers":  {"BillingManagers", "BillingManager"},
		"UnknownRoleType":      {"CustomRole", "CustomRole"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := canonicalizeRoleType(tc.input)
			if diff := cmp.Diff(tc.want, result); diff != "" {
				t.Errorf("canonicalizeRoleType(%q): -want, +got:\n%s", tc.input, diff)
			}
		})
	}
}
