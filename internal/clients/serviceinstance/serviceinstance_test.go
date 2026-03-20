package serviceinstance

import (
	"context"
	"testing"

	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
)

var (
	serviceInstanceGUID              = "service-instance-guid"
	spaceGUID1                       = "space-guid-1"
	spaceGUID2                       = "space-guid-2"
	spaceGUID3                       = "space-guid-3"
	spaceGUID4                       = "space-guid-4"
	spaceGUID5                       = "space-guid-5"
	errorGetSharedSpaceRelationships = "cannot get shared space relationships"
	errorShareWithSpaces             = "cannot share service instance with spaces"
	errorUnShareWithSpaces           = "cannot unshare service instance from spaces"
	apiError                         = "HTTP 500"
)

// matchSpaces returns a matcher that validates a slice contains the expected spaces in any order. This is required because Go randomizes map iteration order.
func matchSpaces(expectedSpaces ...string) func([]string) bool {
	return func(spaces []string) bool {
		if len(spaces) != len(expectedSpaces) {
			return false
		}

		expectedSet := make(map[string]bool, len(expectedSpaces))
		for _, s := range expectedSpaces {
			expectedSet[s] = true
		}

		for _, s := range spaces {
			if !expectedSet[s] {
				return false
			}
		}

		return true
	}
}

func TestAreSharedSpacesUpToDate(t *testing.T) {
	type args struct {
		guid    string
		desired []v1alpha1.SpaceReference
	}

	type want struct {
		result bool
		err    error
	}

	cases := map[string]struct {
		args      args
		want      want
		mockSetup func(*fake.MockServiceInstance)
	}{
		"UpToDate": {
			args: args{
				guid: serviceInstanceGUID,
				desired: []v1alpha1.SpaceReference{
					{Space: ptr.To(spaceGUID1)},
					{Space: ptr.To(spaceGUID2)},
				},
			},
			want: want{
				result: true,
				err:    nil,
			},
			mockSetup: func(m *fake.MockServiceInstance) {
				m.On("GetSharedSpaceRelationships", serviceInstanceGUID).Return(
					&resource.ServiceInstanceSharedSpaceRelationships{
						Data: []resource.Relationship{
							{GUID: spaceGUID1},
							{GUID: spaceGUID2},
						},
					},
					nil,
				)
			},
		},
		"Add": {
			args: args{
				guid: serviceInstanceGUID,
				desired: []v1alpha1.SpaceReference{
					{Space: ptr.To(spaceGUID1)},
					{Space: ptr.To(spaceGUID2)},
					{Space: ptr.To(spaceGUID3)},
				},
			},
			want: want{
				result: false,
				err:    nil,
			},
			mockSetup: func(m *fake.MockServiceInstance) {
				m.On("GetSharedSpaceRelationships", serviceInstanceGUID).Return(
					&resource.ServiceInstanceSharedSpaceRelationships{
						Data: []resource.Relationship{
							{GUID: spaceGUID1},
							{GUID: spaceGUID2},
						},
					},
					nil,
				)
			},
		},
		"Remove": {
			args: args{
				guid: serviceInstanceGUID,
				desired: []v1alpha1.SpaceReference{
					{Space: ptr.To(spaceGUID1)},
				},
			},
			want: want{
				result: false,
				err:    nil,
			},
			mockSetup: func(m *fake.MockServiceInstance) {
				m.On("GetSharedSpaceRelationships", serviceInstanceGUID).Return(
					&resource.ServiceInstanceSharedSpaceRelationships{
						Data: []resource.Relationship{
							{GUID: spaceGUID1},
							{GUID: spaceGUID2},
							{GUID: spaceGUID3},
						},
					},
					nil,
				)
			},
		},
		"AddAndRemove": {
			args: args{
				guid: serviceInstanceGUID,
				desired: []v1alpha1.SpaceReference{
					{Space: ptr.To(spaceGUID2)},
					{Space: ptr.To(spaceGUID4)},
				},
			},
			want: want{
				result: false,
				err:    nil,
			},
			mockSetup: func(m *fake.MockServiceInstance) {
				m.On("GetSharedSpaceRelationships", serviceInstanceGUID).Return(
					&resource.ServiceInstanceSharedSpaceRelationships{
						Data: []resource.Relationship{
							{GUID: spaceGUID1},
							{GUID: spaceGUID2},
							{GUID: spaceGUID3},
						},
					},
					nil,
				)
			},
		},
		"Empty": {
			args: args{
				guid:    serviceInstanceGUID,
				desired: []v1alpha1.SpaceReference{},
			},
			want: want{
				result: true,
				err:    nil,
			},
			mockSetup: func(m *fake.MockServiceInstance) {
				m.On("GetSharedSpaceRelationships", serviceInstanceGUID).Return(
					&resource.ServiceInstanceSharedSpaceRelationships{
						Data: []resource.Relationship{},
					},
					nil,
				)
			},
		},
		"ErrorFromGetSharedSpaceRelationships": {
			args: args{
				guid: serviceInstanceGUID,
				desired: []v1alpha1.SpaceReference{
					{Space: ptr.To(spaceGUID1)},
				},
			},
			want: want{
				result: false,
				err:    errors.New(errorGetSharedSpaceRelationships + ": " + apiError),
			},
			mockSetup: func(m *fake.MockServiceInstance) {
				m.On("GetSharedSpaceRelationships", serviceInstanceGUID).Return(
					nil,
					errors.New(apiError),
				)
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			mockSI := &fake.MockServiceInstance{}
			tc.mockSetup(mockSI)

			client := &Client{
				ServiceInstance: mockSI,
			}

			got, err := AreSharedSpacesUpToDate(context.Background(), client, tc.args.guid, tc.args.desired)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("AreSharedSpacesUpToDate(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("AreSharedSpacesUpToDate(...): want error != got error:\n%s", diff)
				}
			}

			if diff := cmp.Diff(tc.want.result, got); diff != "" {
				t.Errorf("AreSharedSpacesUpToDate(...): -want, +got:\n%s", diff)
			}

			mockSI.AssertExpectations(t)
		})
	}
}

func TestUpdateSharedSpaces(t *testing.T) {
	type args struct {
		guid    string
		desired []v1alpha1.SpaceReference
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		args      args
		want      want
		mockSetup func(*fake.MockServiceInstance)
	}{
		"InSync": {
			args: args{
				guid: serviceInstanceGUID,
				desired: []v1alpha1.SpaceReference{
					{Space: ptr.To(spaceGUID1)},
					{Space: ptr.To(spaceGUID2)},
				},
			},
			want: want{
				err: nil,
			},
			mockSetup: func(m *fake.MockServiceInstance) {
				m.On("GetSharedSpaceRelationships", serviceInstanceGUID).Return(
					&resource.ServiceInstanceSharedSpaceRelationships{
						Data: []resource.Relationship{
							{GUID: spaceGUID1},
							{GUID: spaceGUID2},
						},
					},
					nil,
				)
			},
		},
		"Add": {
			args: args{
				guid: serviceInstanceGUID,
				desired: []v1alpha1.SpaceReference{
					{Space: ptr.To(spaceGUID1)},
					{Space: ptr.To(spaceGUID2)},
					{Space: ptr.To(spaceGUID3)},
				},
			},
			want: want{
				err: nil,
			},
			mockSetup: func(m *fake.MockServiceInstance) {
				m.On("GetSharedSpaceRelationships", serviceInstanceGUID).Return(
					&resource.ServiceInstanceSharedSpaceRelationships{
						Data: []resource.Relationship{
							{GUID: spaceGUID1},
						},
					},
					nil,
				)
				m.On("ShareWithSpaces", serviceInstanceGUID, []string{spaceGUID2, spaceGUID3}).Return(
					&resource.ServiceInstanceSharedSpaceRelationships{}, // can be empty as return value is not used by UpdateSharedSpaces
					nil,
				)
			},
		},
		"Remove": {
			args: args{
				guid: serviceInstanceGUID,
				desired: []v1alpha1.SpaceReference{
					{Space: ptr.To(spaceGUID1)},
				},
			},
			want: want{
				err: nil,
			},
			mockSetup: func(m *fake.MockServiceInstance) {
				m.On("GetSharedSpaceRelationships", serviceInstanceGUID).Return(
					&resource.ServiceInstanceSharedSpaceRelationships{
						Data: []resource.Relationship{
							{GUID: spaceGUID1},
							{GUID: spaceGUID2},
							{GUID: spaceGUID3},
						},
					},
					nil,
				)
				m.On("UnShareWithSpaces", serviceInstanceGUID, mock.MatchedBy(matchSpaces(spaceGUID2, spaceGUID3))).Return(nil)
			},
		},
		"AddAndRemove": {
			args: args{
				guid: serviceInstanceGUID,
				desired: []v1alpha1.SpaceReference{
					{Space: ptr.To(spaceGUID2)},
					{Space: ptr.To(spaceGUID4)},
					{Space: ptr.To(spaceGUID5)},
				},
			},
			want: want{
				err: nil,
			},
			mockSetup: func(m *fake.MockServiceInstance) {
				m.On("GetSharedSpaceRelationships", serviceInstanceGUID).Return(
					&resource.ServiceInstanceSharedSpaceRelationships{
						Data: []resource.Relationship{
							{GUID: spaceGUID1},
							{GUID: spaceGUID2},
							{GUID: spaceGUID3},
						},
					},
					nil,
				)
				m.On("ShareWithSpaces", serviceInstanceGUID, mock.MatchedBy(matchSpaces(spaceGUID4, spaceGUID5))).Return(
					&resource.ServiceInstanceSharedSpaceRelationships{},
					nil,
				)
				m.On("UnShareWithSpaces", serviceInstanceGUID, mock.MatchedBy(matchSpaces(spaceGUID1, spaceGUID3))).Return(nil)
			},
		},
		"ErrorFromGetSharedSpaceRelationships": {
			args: args{
				guid: serviceInstanceGUID,
				desired: []v1alpha1.SpaceReference{
					{Space: ptr.To(spaceGUID1)},
				},
			},
			want: want{
				err: errors.New(errorGetSharedSpaceRelationships + ": " + apiError),
			},
			mockSetup: func(m *fake.MockServiceInstance) {
				m.On("GetSharedSpaceRelationships", serviceInstanceGUID).Return(
					nil,
					errors.New(apiError),
				)
			},
		},
		"ErrorFromShareWithSpaces": {
			args: args{
				guid: serviceInstanceGUID,
				desired: []v1alpha1.SpaceReference{
					{Space: ptr.To(spaceGUID1)},
					{Space: ptr.To(spaceGUID2)},
				},
			},
			want: want{
				err: errors.New(errorShareWithSpaces + ": " + apiError),
			},
			mockSetup: func(m *fake.MockServiceInstance) {
				m.On("GetSharedSpaceRelationships", serviceInstanceGUID).Return(
					&resource.ServiceInstanceSharedSpaceRelationships{
						Data: []resource.Relationship{},
					},
					nil,
				)
				m.On("ShareWithSpaces", serviceInstanceGUID, []string{spaceGUID1, spaceGUID2}).Return(
					nil,
					errors.New(apiError),
				)
			},
		},
		"ErrorFromUnShareWithSpaces": {
			args: args{
				guid:    serviceInstanceGUID,
				desired: []v1alpha1.SpaceReference{},
			},
			want: want{
				err: errors.New(errorUnShareWithSpaces + ": " + apiError),
			},
			mockSetup: func(m *fake.MockServiceInstance) {
				m.On("GetSharedSpaceRelationships", serviceInstanceGUID).Return(
					&resource.ServiceInstanceSharedSpaceRelationships{
						Data: []resource.Relationship{
							{GUID: spaceGUID1},
							{GUID: spaceGUID2},
						},
					},
					nil,
				)
				m.On("UnShareWithSpaces", serviceInstanceGUID, mock.MatchedBy(matchSpaces(spaceGUID1, spaceGUID2))).Return(
					errors.New(apiError),
				)
			},
		},
		"DesiredWithNilAndEmptySpaces": {
			args: args{
				guid: serviceInstanceGUID,
				desired: []v1alpha1.SpaceReference{
					{Space: ptr.To(spaceGUID1)},
					{Space: nil},
					{Space: ptr.To("")},
					{Space: ptr.To(spaceGUID2)},
				},
			},
			want: want{
				err: nil,
			},
			mockSetup: func(m *fake.MockServiceInstance) {
				m.On("GetSharedSpaceRelationships", serviceInstanceGUID).Return(
					&resource.ServiceInstanceSharedSpaceRelationships{
						Data: []resource.Relationship{
							{GUID: spaceGUID1},
							{GUID: spaceGUID2},
						},
					},
					nil,
				)
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			mockSI := &fake.MockServiceInstance{}
			tc.mockSetup(mockSI)

			client := &Client{
				ServiceInstance: mockSI,
			}

			err := client.UpdateSharedSpaces(context.Background(), tc.args.guid, tc.args.desired)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("UpdateSharedSpaces(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("UpdateSharedSpaces(...): want error != got error:\n%s", diff)
				}
			}

			mockSI.AssertExpectations(t)
		})
	}
}
