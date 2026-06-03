package servicecredentialbinding

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/mock"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
)

var (
	errBoom             = errors.New("boom")
	testGUID            = "test-guid-123"
	testServiceInstance = "service-instance-guid"
	testApp             = "app-guid"
	testName            = "test-binding"
)

func TestGetConnectionDetails(t *testing.T) {
	type args struct {
		ctx    context.Context
		client ServiceCredentialBinding
		guid   string
		asJSON bool
	}

	type want struct {
		details managed.ConnectionDetails
	}

	testCredentials := map[string]interface{}{
		"username": "testuser",
		"password": "testpass",
		"nested": map[string]interface{}{
			"key": "value",
		},
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"AsJSONTrue": {
			args: args{
				ctx:    context.Background(),
				client: createMockClientWithDetails(testCredentials, nil),
				guid:   testGUID,
				asJSON: true,
			},
			want: want{
				details: managed.ConnectionDetails{
					"credentials": []byte(`{"nested":{"key":"value"},"password":"testpass","username":"testuser"}`),
				},
			},
		},
		"AsJSONFalse": {
			args: args{
				ctx:    context.Background(),
				client: createMockClientWithDetails(testCredentials, nil),
				guid:   testGUID,
				asJSON: false,
			},
			want: want{
				details: managed.ConnectionDetails{
					"username":   []byte("testuser"),
					"password":   []byte("testpass"),
					"nested_key": []byte("value"),
				},
			},
		},
		"GetDetailsError": {
			args: args{
				ctx:    context.Background(),
				client: createMockClientWithDetails(nil, errBoom),
				guid:   testGUID,
				asJSON: false,
			},
			want: want{
				details: nil,
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			details := GetConnectionDetails(tc.args.ctx, tc.args.client, tc.args.guid, tc.args.asJSON)

			if tc.want.details == nil {
				if details != nil {
					t.Errorf("GetConnectionDetails(...): expected nil, got %v", details)
				}
			} else {
				if diff := cmp.Diff(tc.want.details, details); diff != "" {
					t.Errorf("GetConnectionDetails(...): -want, +got:\n%s", diff)
				}
			}
		})
	}
}

func TestNewListOptions(t *testing.T) {
	type args struct {
		forProvider v1alpha1.ServiceCredentialBindingParameters
	}

	type want struct {
		opts *client.ServiceCredentialBindingListOptions
		err  error
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"KeyBinding": {
			args: args{
				forProvider: v1alpha1.ServiceCredentialBindingParameters{
					Type:            "key",
					Name:            &testName,
					ServiceInstance: &testServiceInstance,
				},
			},
			want: want{
				opts: func() *client.ServiceCredentialBindingListOptions {
					opts := client.NewServiceCredentialBindingListOptions()
					opts.Type.EqualTo("key")
					opts.ServiceInstanceGUIDs.EqualTo(testServiceInstance)
					opts.Names.EqualTo(testName)
					return opts
				}(),
				err: nil,
			},
		},
		"AppBinding": {
			args: args{
				forProvider: v1alpha1.ServiceCredentialBindingParameters{
					Type:            "app",
					App:             &testApp,
					ServiceInstance: &testServiceInstance,
				},
			},
			want: want{
				opts: func() *client.ServiceCredentialBindingListOptions {
					opts := client.NewServiceCredentialBindingListOptions()
					opts.Type.EqualTo("app")
					opts.ServiceInstanceGUIDs.EqualTo(testServiceInstance)
					opts.AppGUIDs.EqualTo(testApp)
					return opts
				}(),
				err: nil,
			},
		},
		"MissingServiceInstance": {
			args: args{
				forProvider: v1alpha1.ServiceCredentialBindingParameters{
					Type: "key",
					Name: &testName,
				},
			},
			want: want{
				opts: nil,
				err:  errors.New(ErrServiceInstanceMissing),
			},
		},
		"MissingAppForAppBinding": {
			args: args{
				forProvider: v1alpha1.ServiceCredentialBindingParameters{
					Type:            "app",
					ServiceInstance: &testServiceInstance,
				},
			},
			want: want{
				opts: nil,
				err:  errors.New(ErrAppMissing),
			},
		},
		"MissingNameForKeyBinding": {
			args: args{
				forProvider: v1alpha1.ServiceCredentialBindingParameters{
					Type:            "key",
					ServiceInstance: &testServiceInstance,
				},
			},
			want: want{
				opts: nil,
				err:  errors.New(ErrNameMissing),
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			opts, err := newListOptions(tc.args.forProvider)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("newListOptions(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("newListOptions(...): want error != got error:\n%s", diff)
				}
			}

			// For successful cases, verify the options are configured correctly
			if tc.want.opts != nil && opts != nil {
				// We can't directly compare the options objects, so we verify key properties
				if opts.Type.Values[0] != tc.want.opts.Type.Values[0] {
					t.Errorf("newListOptions(...): Type mismatch, want %s, got %s", tc.want.opts.Type.Values[0], opts.Type.Values[0])
				}
				if opts.ServiceInstanceGUIDs.Values[0] != tc.want.opts.ServiceInstanceGUIDs.Values[0] {
					t.Errorf("newListOptions(...): ServiceInstanceGUIDs mismatch, want %s, got %s", tc.want.opts.ServiceInstanceGUIDs.Values[0], opts.ServiceInstanceGUIDs.Values[0])
				}
			}
		})
	}
}

func TestNewCreateOption(t *testing.T) {
	type args struct {
		forProvider v1alpha1.ServiceCredentialBindingParameters
		params      json.RawMessage
	}

	type want struct {
		opt *cfresource.ServiceCredentialBindingCreate
		err error
	}

	testParams := json.RawMessage(`{"key": "value"}`)

	cases := map[string]struct {
		args args
		want want
	}{
		"KeyBindingWithParams": {
			args: args{
				forProvider: v1alpha1.ServiceCredentialBindingParameters{
					Type:            "key",
					Name:            &testName,
					ServiceInstance: &testServiceInstance,
				},
				params: testParams,
			},
			want: want{
				opt: func() *cfresource.ServiceCredentialBindingCreate {
					opt := cfresource.NewServiceCredentialBindingCreateKey(testServiceInstance, testName+"-"+randomString(5))
					opt.WithJSONParameters(string(testParams))
					return opt
				}(),
				err: nil,
			},
		},
		"AppBindingWithName": {
			args: args{
				forProvider: v1alpha1.ServiceCredentialBindingParameters{
					Type:            "app",
					Name:            &testName,
					App:             &testApp,
					ServiceInstance: &testServiceInstance,
				},
				params: nil,
			},
			want: want{
				opt: func() *cfresource.ServiceCredentialBindingCreate {
					opt := cfresource.NewServiceCredentialBindingCreateApp(testServiceInstance, testApp)
					opt.WithName(testName)
					return opt
				}(),
				err: nil,
			},
		},
		"AppBindingWithoutName": {
			args: args{
				forProvider: v1alpha1.ServiceCredentialBindingParameters{
					Type:            "app",
					App:             &testApp,
					ServiceInstance: &testServiceInstance,
				},
				params: nil,
			},
			want: want{
				opt: cfresource.NewServiceCredentialBindingCreateApp(testServiceInstance, testApp),
				err: nil,
			},
		},
		"MissingServiceInstance": {
			args: args{
				forProvider: v1alpha1.ServiceCredentialBindingParameters{
					Type: "key",
					Name: &testName,
				},
				params: nil,
			},
			want: want{
				opt: nil,
				err: errors.New(ErrServiceInstanceMissing),
			},
		},
		"MissingNameForKey": {
			args: args{
				forProvider: v1alpha1.ServiceCredentialBindingParameters{
					Type:            "key",
					ServiceInstance: &testServiceInstance,
				},
				params: nil,
			},
			want: want{
				opt: nil,
				err: errors.New(ErrNameMissing),
			},
		},
		"MissingAppForApp": {
			args: args{
				forProvider: v1alpha1.ServiceCredentialBindingParameters{
					Type:            "app",
					ServiceInstance: &testServiceInstance,
				},
				params: nil,
			},
			want: want{
				opt: nil,
				err: errors.New(ErrAppMissing),
			},
		},
		"UnknownBindingType": {
			args: args{
				forProvider: v1alpha1.ServiceCredentialBindingParameters{
					Type:            "unknown",
					ServiceInstance: &testServiceInstance,
				},
				params: nil,
			},
			want: want{
				opt: nil,
				err: errors.New(ErrBindingTypeUnknown),
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			opt, err := newCreateOption(tc.args.forProvider, tc.args.params)

			if tc.want.err != nil && err != nil {
				if diff := cmp.Diff(tc.want.err.Error(), err.Error()); diff != "" {
					t.Errorf("newCreateOption(...): want error string != got error string:\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err); diff != "" {
					t.Errorf("newCreateOption(...): want error != got error:\n%s", diff)
				}
			}

			// For successful cases, verify basic properties
			if tc.want.opt != nil && opt != nil {
				if opt.Type != tc.want.opt.Type {
					t.Errorf("newCreateOption(...): Type mismatch, want %s, got %s", tc.want.opt.Type, opt.Type)
				}
				if opt.Relationships.ServiceInstance.Data.GUID != tc.want.opt.Relationships.ServiceInstance.Data.GUID {
					t.Errorf("newCreateOption(...): ServiceInstance GUID mismatch")
				}
			}
		})
	}
}

func TestUpdateObservation(t *testing.T) {
	observation := &v1alpha1.ServiceCredentialBindingObservation{}

	now := time.Now()
	resource := &cfresource.ServiceCredentialBinding{
		Resource: cfresource.Resource{GUID: testGUID},
		LastOperation: cfresource.LastOperation{
			Type:        v1alpha1.LastOperationCreate,
			State:       v1alpha1.LastOperationSucceeded,
			Description: "Create succeeded",
			UpdatedAt:   now,
			CreatedAt:   now,
		},
	}

	UpdateObservation(observation, resource)

	if observation.GUID != testGUID {
		t.Errorf("UpdateObservation(...): GUID mismatch, want %s, got %s", testGUID, observation.GUID)
	}
	if observation.LastOperation.Type != v1alpha1.LastOperationCreate {
		t.Errorf("UpdateObservation(...): LastOperation.Type mismatch")
	}
	if observation.LastOperation.State != v1alpha1.LastOperationSucceeded {
		t.Errorf("UpdateObservation(...): LastOperation.State mismatch")
	}
}

func TestIsUpToDate(t *testing.T) {
	forProvider := v1alpha1.ServiceCredentialBindingParameters{
		Type: "key",
		Name: &testName,
	}

	resource := cfresource.ServiceCredentialBinding{
		Resource: cfresource.Resource{GUID: testGUID},
	}

	// Currently always returns true as per implementation
	result := IsUpToDate(context.Background(), forProvider, resource)
	if !result {
		t.Errorf("IsUpToDate(...): expected true, got false")
	}
}

// Helper function to create mock client with details
func createMockClientWithDetails(credentials map[string]interface{}, err error) ServiceCredentialBinding {
	mockClient := &fake.MockServiceCredentialBinding{}

	if err != nil {
		mockClient.On("GetDetails", mock.Anything, testGUID).Return(
			(*cfresource.ServiceCredentialBindingDetails)(nil),
			err,
		)
	} else {
		details := &cfresource.ServiceCredentialBindingDetails{
			Credentials: credentials,
		}
		mockClient.On("GetDetails", mock.Anything, testGUID).Return(details, nil)
	}

	return mockClient
}
