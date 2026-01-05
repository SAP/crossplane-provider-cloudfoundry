package serviceroutebinding

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/mock"

	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/fake"
)

var (
	errBoom             = errors.New("boom")
	testGUID            = "test-guid-123"
	testRouteGUID       = "route-guid-456"
	testServiceInstance = "service-instance-guid-789"
	testRouteServiceURL = "https://route-service.example.com"
)

func TestNewCreateOption(t *testing.T) {
	type args struct {
		forProvider          v1alpha1.ServiceRouteBindingParameters
		parametersFromSecret runtime.RawExtension
	}

	type want struct {
		opt *cfresource.ServiceRouteBindingCreate
		err error
	}

	testParams := json.RawMessage(`{"key": "value"}`)

	cases := map[string]struct {
		args args
		want want
	}{
		"WithInlineParameters": {
			args: args{
				forProvider: v1alpha1.ServiceRouteBindingParameters{
					RouteReference: v1alpha1.RouteReference{
						Route: testRouteGUID,
					},
					ServiceInstanceReference: v1alpha1.ServiceInstanceReference{
						ServiceInstance: testServiceInstance,
					},
					Parameters: runtime.RawExtension{Raw: testParams},
				},
				parametersFromSecret: runtime.RawExtension{},
			},
			want: want{
				opt: func() *cfresource.ServiceRouteBindingCreate {
					opt := cfresource.NewServiceRouteBindingCreate(testRouteGUID, testServiceInstance)
					opt.Parameters = (*json.RawMessage)(&testParams)
					return opt
				}(),
				err: nil,
			},
		},
		"WithSecretParameters": {
			args: args{
				forProvider: v1alpha1.ServiceRouteBindingParameters{
					RouteReference: v1alpha1.RouteReference{
						Route: testRouteGUID,
					},
					ServiceInstanceReference: v1alpha1.ServiceInstanceReference{
						ServiceInstance: testServiceInstance,
					},
				},
				parametersFromSecret: runtime.RawExtension{Raw: testParams},
			},
			want: want{
				opt: func() *cfresource.ServiceRouteBindingCreate {
					opt := cfresource.NewServiceRouteBindingCreate(testRouteGUID, testServiceInstance)
					opt.Parameters = (*json.RawMessage)(&testParams)
					return opt
				}(),
				err: nil,
			},
		},
		"WithoutParameters": {
			args: args{
				forProvider: v1alpha1.ServiceRouteBindingParameters{
					RouteReference: v1alpha1.RouteReference{
						Route: testRouteGUID,
					},
					ServiceInstanceReference: v1alpha1.ServiceInstanceReference{
						ServiceInstance: testServiceInstance,
					},
				},
			},
			want: want{
				opt: cfresource.NewServiceRouteBindingCreate(testRouteGUID, testServiceInstance),
				err: nil,
			},
		},
		"InlineParametersPriority": {
			args: args{
				forProvider: v1alpha1.ServiceRouteBindingParameters{
					RouteReference: v1alpha1.RouteReference{
						Route: testRouteGUID,
					},
					ServiceInstanceReference: v1alpha1.ServiceInstanceReference{
						ServiceInstance: testServiceInstance,
					},
					Parameters: runtime.RawExtension{Raw: testParams},
				},
				parametersFromSecret: runtime.RawExtension{Raw: json.RawMessage(`{"other": "data"}`)},
			},
			want: want{
				opt: func() *cfresource.ServiceRouteBindingCreate {
					opt := cfresource.NewServiceRouteBindingCreate(testRouteGUID, testServiceInstance)
					opt.Parameters = (*json.RawMessage)(&testParams)
					return opt
				}(),
				err: nil,
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			opt := newCreateOption(tc.args.forProvider, tc.args.parametersFromSecret)

			if tc.want.opt != nil && opt != nil {
				if opt.Relationships.Route.Data.GUID != tc.want.opt.Relationships.Route.Data.GUID {
					t.Errorf("newCreateOption(...): Route GUID mismatch, want %s, got %s",
						tc.want.opt.Relationships.Route.Data.GUID, opt.Relationships.Route.Data.GUID)
				}
				if opt.Relationships.ServiceInstance.Data.GUID != tc.want.opt.Relationships.ServiceInstance.Data.GUID {
					t.Errorf("newCreateOption(...): ServiceInstance GUID mismatch, want %s, got %s",
						tc.want.opt.Relationships.ServiceInstance.Data.GUID, opt.Relationships.ServiceInstance.Data.GUID)
				}
			}
		})
	}
}

func TestUpdateObservation(t *testing.T) {
	type args struct {
		resource           *cfresource.ServiceRouteBinding
		externalParameters *runtime.RawExtension
	}

	type want struct {
		observation v1alpha1.ServiceRouteBindingObservation
	}

	now := time.Now()
	testParams := runtime.RawExtension{Raw: json.RawMessage(`{"key": "value"}`)}
	testLabel := "label"
	testAnnotation := "annotation"

	cases := map[string]struct {
		args args
		want want
	}{
		"CompleteObservation": {
			args: args{
				resource: &cfresource.ServiceRouteBinding{
					Resource: cfresource.Resource{
						GUID:      testGUID,
						CreatedAt: now,
					},
					Metadata: &cfresource.Metadata{
						Labels:      map[string]*string{"test": &testLabel},
						Annotations: map[string]*string{"test": &testAnnotation},
					},
					LastOperation: cfresource.LastOperation{
						Type:        v1alpha1.LastOperationCreate,
						State:       v1alpha1.LastOperationSucceeded,
						Description: "Create succeeded",
						UpdatedAt:   now,
						CreatedAt:   now,
					},
					RouteServiceURL: testRouteServiceURL,
					Relationships: cfresource.ServiceRouteBindingRelationships{
						Route: cfresource.ToOneRelationship{
							Data: &cfresource.Relationship{GUID: testRouteGUID},
						},
						ServiceInstance: cfresource.ToOneRelationship{
							Data: &cfresource.Relationship{GUID: testServiceInstance},
						},
					},
				},
				externalParameters: &testParams,
			},
			want: want{
				observation: v1alpha1.ServiceRouteBindingObservation{
					Resource: v1alpha1.Resource{
						GUID: testGUID,
					},
					RouteServiceUrl: testRouteServiceURL,
					Route:           testRouteGUID,
					ServiceInstance: testServiceInstance,
					LastOperation: &v1alpha1.LastOperation{
						Type:  v1alpha1.LastOperationCreate,
						State: v1alpha1.LastOperationSucceeded,
					},
					ResourceMetadata: v1alpha1.ResourceMetadata{
						Labels:      map[string]*string{"test": &testLabel},
						Annotations: map[string]*string{"test": &testAnnotation},
					},
					Parameters: testParams,
				},
			},
		},
		"WithoutParameters": {
			args: args{
				resource: &cfresource.ServiceRouteBinding{
					Resource: cfresource.Resource{
						GUID:      testGUID,
						CreatedAt: now,
					},
					Metadata: &cfresource.Metadata{},
					LastOperation: cfresource.LastOperation{
						Type:        v1alpha1.LastOperationCreate,
						State:       v1alpha1.LastOperationSucceeded,
						Description: "Create succeeded",
						UpdatedAt:   now,
						CreatedAt:   now,
					},
					RouteServiceURL: testRouteServiceURL,
					Relationships: cfresource.ServiceRouteBindingRelationships{
						Route: cfresource.ToOneRelationship{
							Data: &cfresource.Relationship{GUID: testRouteGUID},
						},
						ServiceInstance: cfresource.ToOneRelationship{
							Data: &cfresource.Relationship{GUID: testServiceInstance},
						},
					},
				},
				externalParameters: &runtime.RawExtension{},
			},
			want: want{
				observation: v1alpha1.ServiceRouteBindingObservation{
					Resource: v1alpha1.Resource{
						GUID: testGUID,
					},
					RouteServiceUrl: testRouteServiceURL,
					Route:           testRouteGUID,
					ServiceInstance: testServiceInstance,
					LastOperation: &v1alpha1.LastOperation{
						Type:  v1alpha1.LastOperationCreate,
						State: v1alpha1.LastOperationSucceeded,
					},
					Parameters: runtime.RawExtension{},
				},
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			observation := &v1alpha1.ServiceRouteBindingObservation{}
			UpdateObservation(observation, tc.args.resource, tc.args.externalParameters)

			if observation.GUID != tc.want.observation.GUID {
				t.Errorf("UpdateObservation(...): GUID mismatch, want %s, got %s",
					tc.want.observation.GUID, observation.GUID)
			}
			if observation.LastOperation.Type != tc.want.observation.LastOperation.Type {
				t.Errorf("UpdateObservation(...): LastOperation.Type mismatch, want %s, got %s",
					tc.want.observation.LastOperation.Type, observation.LastOperation.Type)
			}
			if observation.LastOperation.State != tc.want.observation.LastOperation.State {
				t.Errorf("UpdateObservation(...): LastOperation.State mismatch, want %s, got %s",
					tc.want.observation.LastOperation.State, observation.LastOperation.State)
			}
			if observation.RouteServiceUrl != tc.want.observation.RouteServiceUrl {
				t.Errorf("UpdateObservation(...): RouteServiceUrl mismatch, want %s, got %s",
					tc.want.observation.RouteServiceUrl, observation.RouteServiceUrl)
			}
			if observation.Route != tc.want.observation.Route {
				t.Errorf("UpdateObservation(...): Route GUID mismatch, want %s, got %s",
					tc.want.observation.Route, observation.Route)
			}
			if observation.ServiceInstance != tc.want.observation.ServiceInstance {
				t.Errorf("UpdateObservation(...): ServiceInstance GUID mismatch, want %s, got %s",
					tc.want.observation.ServiceInstance, observation.ServiceInstance)
			}
			if diff := cmp.Diff(tc.want.observation.ResourceMetadata, observation.ResourceMetadata); diff != "" {
				t.Errorf("UpdateObservation(...): ResourceMetadata mismatch, -want +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.observation.Parameters, observation.Parameters); diff != "" {
				t.Errorf("UpdateObservation(...): Parameters mismatch, -want +got:\n%s", diff)
			}
		})
	}
}

func TestBuildLinks(t *testing.T) {
	type args struct {
		cfLinks cfresource.Links
	}

	type want struct {
		links v1alpha1.Links
	}

	getMethod := "GET"
	deleteMethod := "DELETE"

	cases := map[string]struct {
		args args
		want want
	}{
		"NilLinks": {
			args: args{
				cfLinks: nil,
			},
			want: want{
				links: v1alpha1.Links{},
			},
		},
		"EmptyLinks": {
			args: args{
				cfLinks: cfresource.Links{},
			},
			want: want{
				links: v1alpha1.Links{},
			},
		},
		"SelfLinkOnly": {
			args: args{
				cfLinks: cfresource.Links{
					"self": cfresource.Link{
						Href:   "https://api.cf.example.com/v3/service_route_bindings/guid",
						Method: getMethod,
					},
				},
			},
			want: want{
				links: v1alpha1.Links{
					"self": v1alpha1.Link{
						Href:   "https://api.cf.example.com/v3/service_route_bindings/guid",
						Method: &getMethod,
					},
				},
			},
		},
		"MultipleLinks": {
			args: args{
				cfLinks: cfresource.Links{
					"self": cfresource.Link{
						Href:   "https://api.cf.example.com/v3/service_route_bindings/guid",
						Method: getMethod,
					},
					"route": cfresource.Link{
						Href: "https://api.cf.example.com/v3/routes/route-guid",
					},
					"service_instance": cfresource.Link{
						Href:   "https://api.cf.example.com/v3/service_instances/si-guid",
						Method: deleteMethod,
					},
				},
			},
			want: want{
				links: v1alpha1.Links{
					"self": v1alpha1.Link{
						Href:   "https://api.cf.example.com/v3/service_route_bindings/guid",
						Method: &getMethod,
					},
					"route": v1alpha1.Link{
						Href:   "https://api.cf.example.com/v3/routes/route-guid",
						Method: nil,
					},
					"service_instance": v1alpha1.Link{
						Href:   "https://api.cf.example.com/v3/service_instances/si-guid",
						Method: &deleteMethod,
					},
				},
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			links := buildLinks(tc.args.cfLinks)

			if diff := cmp.Diff(tc.want.links, links); diff != "" {
				t.Errorf("buildLinks(...): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestGetParameters(t *testing.T) {
	type args struct {
		ctx       context.Context
		srbClient ServiceRouteBinding
		guid      string
	}

	type want struct {
		params *runtime.RawExtension
		err    error
	}

	testParamsMap := map[string]string{
		"timeout": "30",
		"retries": "3",
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"Success": {
			args: args{
				ctx:       context.Background(),
				srbClient: createMockClientWithParameters(testParamsMap, nil),
				guid:      testGUID,
			},
			want: want{
				params: func() *runtime.RawExtension {
					jsonBytes, err := json.Marshal(testParamsMap)
					if err != nil {
						t.Fatalf("Failed to marshal testParamsMap: %v", err)
					}
					return &runtime.RawExtension{Raw: jsonBytes}
				}(),
				err: nil,
			},
		},
		"GetParametersError": {
			args: args{
				ctx:       context.Background(),
				srbClient: createMockClientWithParameters(nil, errBoom),
				guid:      testGUID,
			},
			want: want{
				params: nil,
				err:    errBoom,
			},
		},
		"EmptyParameters": {
			args: args{
				ctx:       context.Background(),
				srbClient: createMockClientWithParameters(map[string]string{}, nil),
				guid:      testGUID,
			},
			want: want{
				params: func() *runtime.RawExtension {
					jsonBytes, err := json.Marshal(map[string]string{})
					if err != nil {
						t.Fatalf("Failed to marshal empty map: %v", err)
					}
					return &runtime.RawExtension{Raw: jsonBytes}
				}(),
				err: nil,
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			params, err := GetParameters(tc.args.ctx, tc.args.srbClient, tc.args.guid)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("GetParameters(...): expected error %v, got nil", tc.want.err)
				} else if !errors.Is(err, tc.want.err) && err.Error() != tc.want.err.Error() {
					t.Errorf("GetParameters(...): expected error %v, got %v", tc.want.err, err)
				}
			} else {
				if err != nil {
					t.Errorf("GetParameters(...): unexpected error: %v", err)
				}
				if diff := cmp.Diff(tc.want.params, params); diff != "" {
					t.Errorf("GetParameters(...): -want, +got:\n%s", diff)
				}
			}
		})
	}
}

func TestCreateToListOptions(t *testing.T) {
	create := cfresource.NewServiceRouteBindingCreate(testRouteGUID, testServiceInstance)

	opts := createToListOptions(create)

	if opts == nil {
		t.Fatal("createToListOptions(...): returned nil")
	}
	if len(opts.RouteGUIDs.Values) != 1 || opts.RouteGUIDs.Values[0] != testRouteGUID {
		t.Errorf("createToListOptions(...): RouteGUIDs not set correctly, got %v", opts.RouteGUIDs.Values)
	}
	if len(opts.ServiceInstanceGUIDs.Values) != 1 || opts.ServiceInstanceGUIDs.Values[0] != testServiceInstance {
		t.Errorf("createToListOptions(...): ServiceInstanceGUIDs not set correctly, got %v", opts.ServiceInstanceGUIDs.Values)
	}
}

func TestGetByID(t *testing.T) {
	type args struct {
		ctx         context.Context
		srbClient   ServiceRouteBinding
		guid        string
		forProvider v1alpha1.ServiceRouteBindingParameters
	}

	type want struct {
		binding     *cfresource.ServiceRouteBinding
		err         error
		expectError bool
	}

	validGUID := "550e8400-e29b-41d4-a716-446655440000"
	invalidGUID := "not-a-valid-guid"

	testBinding := &cfresource.ServiceRouteBinding{
		Resource: cfresource.Resource{
			GUID: validGUID,
		},
		RouteServiceURL: testRouteServiceURL,
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"GetByValidGUID_Success": {
			args: args{
				ctx:  context.Background(),
				guid: validGUID,
				srbClient: func() ServiceRouteBinding {
					mockClient := &fake.MockServiceRouteBinding{}
					mockClient.On("Get", mock.Anything, validGUID).Return(testBinding, nil)
					return mockClient
				}(),
				forProvider: v1alpha1.ServiceRouteBindingParameters{},
			},
			want: want{
				binding:     testBinding,
				err:         nil,
				expectError: false,
			},
		},
		"GetByValidGUID_Error": {
			args: args{
				ctx:  context.Background(),
				guid: validGUID,
				srbClient: func() ServiceRouteBinding {
					mockClient := &fake.MockServiceRouteBinding{}
					mockClient.On("Get", mock.Anything, validGUID).Return(nil, errBoom)
					return mockClient
				}(),
				forProvider: v1alpha1.ServiceRouteBindingParameters{},
			},
			want: want{
				binding:     nil,
				err:         errBoom,
				expectError: true,
			},
		},
		"InvalidGUID_ReturnsError": {
			args: args{
				ctx:       context.Background(),
				guid:      invalidGUID,
				srbClient: &fake.MockServiceRouteBinding{},
				forProvider: v1alpha1.ServiceRouteBindingParameters{
					RouteReference: v1alpha1.RouteReference{
						Route: testRouteGUID,
					},
					ServiceInstanceReference: v1alpha1.ServiceInstanceReference{
						ServiceInstance: testServiceInstance,
					},
				},
			},
			want: want{
				binding:     nil,
				err:         nil,
				expectError: true,
			},
		},
		"EmptyGUID_ReturnsError": {
			args: args{
				ctx:       context.Background(),
				guid:      "",
				srbClient: &fake.MockServiceRouteBinding{},
				forProvider: v1alpha1.ServiceRouteBindingParameters{
					RouteReference: v1alpha1.RouteReference{
						Route: testRouteGUID,
					},
					ServiceInstanceReference: v1alpha1.ServiceInstanceReference{
						ServiceInstance: testServiceInstance,
					},
				},
			},
			want: want{
				binding:     nil,
				err:         nil,
				expectError: true,
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			binding, err := GetByID(tc.args.ctx, tc.args.srbClient, tc.args.guid, tc.args.forProvider)

			if tc.want.expectError {
				if err == nil {
					t.Errorf("GetByID(...): expected an error, got nil")
				}
				if tc.want.err != nil && !errors.Is(err, tc.want.err) && err.Error() != tc.want.err.Error() {
					t.Errorf("GetByID(...): expected error %v, got %v", tc.want.err, err)
				}
			} else {
				if err != nil {
					t.Errorf("GetByID(...): unexpected error: %v", err)
				}
				if diff := cmp.Diff(tc.want.binding, binding); diff != "" {
					t.Errorf("GetByID(...): -want, +got:\n%s", diff)
				}
				// Verify that the returned binding has the correct GUID
				if binding != nil && binding.GUID != tc.args.guid {
					t.Errorf("GetByID(...): expected binding with GUID %s, got %s", tc.args.guid, binding.GUID)
				}
			}
		})
	}
}

func TestDelete(t *testing.T) {
	type args struct {
		ctx       context.Context
		srbClient ServiceRouteBinding
		guid      string
	}

	type want struct {
		err error
	}

	testJobGUID := "job-guid-123"

	cases := map[string]struct {
		args args
		want want
	}{
		"SuccessWithJob": {
			args: args{
				ctx:  context.Background(),
				guid: testGUID,
				srbClient: func() ServiceRouteBinding {
					mockClient := &fake.MockServiceRouteBinding{}
					mockClient.On("Delete", mock.Anything, testGUID).Return(testJobGUID, nil)
					mockClient.On("PollComplete", mock.Anything, testJobGUID, mock.Anything).Return(nil)
					return mockClient
				}(),
			},
			want: want{
				err: nil,
			},
		},
		"SuccessWithoutJob": {
			args: args{
				ctx:  context.Background(),
				guid: testGUID,
				srbClient: func() ServiceRouteBinding {
					mockClient := &fake.MockServiceRouteBinding{}
					mockClient.On("Delete", mock.Anything, testGUID).Return("", nil)
					return mockClient
				}(),
			},
			want: want{
				err: nil,
			},
		},
		"DeleteError": {
			args: args{
				ctx:  context.Background(),
				guid: testGUID,
				srbClient: func() ServiceRouteBinding {
					mockClient := &fake.MockServiceRouteBinding{}
					mockClient.On("Delete", mock.Anything, testGUID).Return("", errBoom)
					return mockClient
				}(),
			},
			want: want{
				err: errBoom,
			},
		},
		"PollJobError": {
			args: args{
				ctx:  context.Background(),
				guid: testGUID,
				srbClient: func() ServiceRouteBinding {
					mockClient := &fake.MockServiceRouteBinding{}
					mockClient.On("Delete", mock.Anything, testGUID).Return(testJobGUID, nil)
					mockClient.On("PollComplete", mock.Anything, testJobGUID, mock.Anything).Return(errBoom)
					return mockClient
				}(),
			},
			want: want{
				err: errBoom,
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			err := Delete(tc.args.ctx, tc.args.srbClient, tc.args.guid)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("Delete(...): expected error %v, got nil", tc.want.err)
				} else if !errors.Is(err, tc.want.err) && err.Error() != tc.want.err.Error() {
					t.Errorf("Delete(...): expected error %v, got %v", tc.want.err, err)
				}
			} else {
				if err != nil {
					t.Errorf("Delete(...): unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCreate(t *testing.T) {
	type args struct {
		ctx                  context.Context
		srbClient            ServiceRouteBinding
		forProvider          v1alpha1.ServiceRouteBindingParameters
		parametersFromSecret runtime.RawExtension
	}

	type want struct {
		binding *cfresource.ServiceRouteBinding
		err     error
	}

	testJobGUID := "job-guid-123"
	testParams := json.RawMessage(`{"key": "value"}`)

	testBinding := &cfresource.ServiceRouteBinding{
		Resource: cfresource.Resource{
			GUID: testGUID,
		},
		RouteServiceURL: testRouteServiceURL,
		Relationships: cfresource.ServiceRouteBindingRelationships{
			Route: cfresource.ToOneRelationship{
				Data: &cfresource.Relationship{GUID: testRouteGUID},
			},
			ServiceInstance: cfresource.ToOneRelationship{
				Data: &cfresource.Relationship{GUID: testServiceInstance},
			},
		},
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"SuccessWithJob": {
			args: args{
				ctx: context.Background(),
				forProvider: v1alpha1.ServiceRouteBindingParameters{
					RouteReference: v1alpha1.RouteReference{
						Route: testRouteGUID,
					},
					ServiceInstanceReference: v1alpha1.ServiceInstanceReference{
						ServiceInstance: testServiceInstance,
					},
				},
				parametersFromSecret: runtime.RawExtension{},
				srbClient: func() ServiceRouteBinding {
					mockClient := &fake.MockServiceRouteBinding{}
					mockClient.On("Create", mock.Anything, mock.MatchedBy(func(opt *cfresource.ServiceRouteBindingCreate) bool {
						return opt.Relationships.Route.Data.GUID == testRouteGUID &&
							opt.Relationships.ServiceInstance.Data.GUID == testServiceInstance
					})).Return(testJobGUID, testBinding, nil)
					mockClient.On("PollComplete", mock.Anything, testJobGUID, mock.Anything).Return(nil)
					mockClient.On("Single", mock.Anything, mock.Anything).Return(testBinding, nil)
					return mockClient
				}(),
			},
			want: want{
				binding: testBinding,
				err:     nil,
			},
		},
		"SuccessWithoutJob": {
			args: args{
				ctx: context.Background(),
				forProvider: v1alpha1.ServiceRouteBindingParameters{
					RouteReference: v1alpha1.RouteReference{
						Route: testRouteGUID,
					},
					ServiceInstanceReference: v1alpha1.ServiceInstanceReference{
						ServiceInstance: testServiceInstance,
					},
				},
				parametersFromSecret: runtime.RawExtension{},
				srbClient: func() ServiceRouteBinding {
					mockClient := &fake.MockServiceRouteBinding{}
					mockClient.On("Create", mock.Anything, mock.Anything).Return("", testBinding, nil)
					mockClient.On("Single", mock.Anything, mock.Anything).Return(testBinding, nil)
					return mockClient
				}(),
			},
			want: want{
				binding: testBinding,
				err:     nil,
			},
		},
		"SuccessWithParameters": {
			args: args{
				ctx: context.Background(),
				forProvider: v1alpha1.ServiceRouteBindingParameters{
					RouteReference: v1alpha1.RouteReference{
						Route: testRouteGUID,
					},
					ServiceInstanceReference: v1alpha1.ServiceInstanceReference{
						ServiceInstance: testServiceInstance,
					},
					Parameters: runtime.RawExtension{Raw: testParams},
				},
				parametersFromSecret: runtime.RawExtension{},
				srbClient: func() ServiceRouteBinding {
					mockClient := &fake.MockServiceRouteBinding{}
					mockClient.On("Create", mock.Anything, mock.MatchedBy(func(opt *cfresource.ServiceRouteBindingCreate) bool {
						return opt.Parameters != nil
					})).Return("", testBinding, nil)
					mockClient.On("Single", mock.Anything, mock.Anything).Return(testBinding, nil)
					return mockClient
				}(),
			},
			want: want{
				binding: testBinding,
				err:     nil,
			},
		},
		"CreateError": {
			args: args{
				ctx: context.Background(),
				forProvider: v1alpha1.ServiceRouteBindingParameters{
					RouteReference: v1alpha1.RouteReference{
						Route: testRouteGUID,
					},
					ServiceInstanceReference: v1alpha1.ServiceInstanceReference{
						ServiceInstance: testServiceInstance,
					},
				},
				parametersFromSecret: runtime.RawExtension{},
				srbClient: func() ServiceRouteBinding {
					mockClient := &fake.MockServiceRouteBinding{}
					mockClient.On("Create", mock.Anything, mock.Anything).Return("", testBinding, errBoom)
					return mockClient
				}(),
			},
			want: want{
				binding: testBinding,
				err:     errBoom,
			},
		},
		"PollJobError": {
			args: args{
				ctx: context.Background(),
				forProvider: v1alpha1.ServiceRouteBindingParameters{
					RouteReference: v1alpha1.RouteReference{
						Route: testRouteGUID,
					},
					ServiceInstanceReference: v1alpha1.ServiceInstanceReference{
						ServiceInstance: testServiceInstance,
					},
				},
				parametersFromSecret: runtime.RawExtension{},
				srbClient: func() ServiceRouteBinding {
					mockClient := &fake.MockServiceRouteBinding{}
					mockClient.On("Create", mock.Anything, mock.Anything).Return(testJobGUID, testBinding, nil)
					mockClient.On("PollComplete", mock.Anything, testJobGUID, mock.Anything).Return(errBoom)
					return mockClient
				}(),
			},
			want: want{
				binding: nil,
				err:     errBoom,
			},
		},
		"SingleError": {
			args: args{
				ctx: context.Background(),
				forProvider: v1alpha1.ServiceRouteBindingParameters{
					RouteReference: v1alpha1.RouteReference{
						Route: testRouteGUID,
					},
					ServiceInstanceReference: v1alpha1.ServiceInstanceReference{
						ServiceInstance: testServiceInstance,
					},
				},
				parametersFromSecret: runtime.RawExtension{},
				srbClient: func() ServiceRouteBinding {
					mockClient := &fake.MockServiceRouteBinding{}
					mockClient.On("Create", mock.Anything, mock.Anything).Return(testJobGUID, testBinding, nil)
					mockClient.On("PollComplete", mock.Anything, testJobGUID, mock.Anything).Return(nil)
					mockClient.On("Single", mock.Anything, mock.Anything).Return(nil, errBoom)
					return mockClient
				}(),
			},
			want: want{
				binding: nil,
				err:     errBoom,
			},
		},
	}

	for n, tc := range cases {
		t.Run(n, func(t *testing.T) {
			binding, err := Create(tc.args.ctx, tc.args.srbClient, tc.args.forProvider, tc.args.parametersFromSecret)

			if tc.want.err != nil {
				if err == nil {
					t.Errorf("Create(...): expected error %v, got nil", tc.want.err)
				} else if !errors.Is(err, tc.want.err) && err.Error() != tc.want.err.Error() {
					t.Errorf("Create(...): expected error %v, got %v", tc.want.err, err)
				}
			} else {
				if err != nil {
					t.Errorf("Create(...): unexpected error: %v", err)
				}
				if diff := cmp.Diff(tc.want.binding, binding); diff != "" {
					t.Errorf("Create(...): -want, +got:\n%s", diff)
				}
			}
		})
	}
}

// Helper function to create mock client with parameters
func createMockClientWithParameters(params map[string]string, err error) ServiceRouteBinding {
	mockClient := &fake.MockServiceRouteBinding{}

	mockClient.On("GetParameters", mock.Anything, testGUID).Return(params, err)

	return mockClient
}
