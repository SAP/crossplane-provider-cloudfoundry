//go:build e2e

package e2e

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/envconf"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis"
	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	testutil "github.com/SAP/crossplane-provider-cloudfoundry/test"
)

const rotationSetupPhaseTimeout = 5 * time.Minute

func newRotationSetupOperations(t *testing.T, cfg *envconf.Config, fixtureDir, bindingName, instanceName, orgName, spaceName string) testutil.RotationSetupOperations {
	t.Helper()
	resourceClient := cfg.Client().Resources()
	return testutil.RotationSetupOperations{
		ApplyImports: func(ctx context.Context) error {
			return applyRotationFixtureFile(ctx, cfg, fixtureDir, "import.yaml")
		},
		ImportsReady: func(ctx context.Context) (bool, error) {
			org := &v1alpha1.Organization{}
			space := &v1alpha1.Space{}
			if err := resourceClient.Get(ctx, runScopedName(orgName), "", org); err != nil {
				return false, err
			}
			if err := resourceClient.Get(ctx, runScopedName(spaceName), "", space); err != nil {
				return false, err
			}
			return testutil.RotationOrganizationSpaceReady(org, space)
		},
		ApplyInstance: func(ctx context.Context) error {
			return applyRotationFixtureFile(ctx, cfg, fixtureDir, "serviceinstance.yaml")
		},
		InstanceReady: func(ctx context.Context) (bool, error) {
			instance := &v1alpha1.ServiceInstance{}
			if err := resourceClient.Get(ctx, runScopedName(instanceName), "", instance); err != nil {
				return false, err
			}
			return testutil.RotationServiceInstanceReady(instance)
		},
		ApplyBinding: func(ctx context.Context) error {
			return applyRotationFixtureFile(ctx, cfg, fixtureDir, "scb.yaml")
		},
		BindingReady: func(ctx context.Context) (bool, error) {
			binding := &v1alpha1.ServiceCredentialBinding{}
			if err := resourceClient.Get(ctx, runScopedName(bindingName), "", binding); err != nil {
				return false, err
			}
			return testutil.RotationServiceCredentialBindingReady(binding)
		},
		PhaseTimeout: rotationSetupPhaseTimeout,
		PollInterval: 5 * time.Second,
		OnProgress: func(phase, state string, elapsed time.Duration) {
			t.Logf("rotation setup phase=%s state=%s elapsed=%s", phase, state, elapsed.Round(time.Second))
		},
		OnFailureRead: func(ctx context.Context, phase string) {
			logRotationSetupDiagnostic(t, ctx, resourceClient, phase, bindingName, instanceName, orgName, spaceName)
		},
	}
}

// applyRotationFixtureFile decodes and applies exactly one known manifest. An
// existing run-scoped object is accepted only if its GVK, name, namespace, and
// desired spec match; it is never silently adopted or rewritten.
func applyRotationFixtureFile(ctx context.Context, cfg *envconf.Config, dir, filename string) error {
	client, err := resources.New(cfg.Client().RESTConfig())
	if err != nil {
		return testutil.RotationSetupApplyFailure(err, "FixtureFile", filename)
	}
	if err := apis.AddToScheme(client.GetScheme()); err != nil {
		return &testutil.RotationSetupError{Kind: testutil.RotationSetupFixtureKindMissing, ResourceKind: "FixtureFile", ResourceName: filename}
	}
	client.WithNamespace(cfg.Namespace())
	seen := false
	err = decoder.DecodeEachFile(ctx, os.DirFS(dir), filename, func(ctx context.Context, desired k8s.Object) error {
		seen = true
		return applyRotationFixtureDocument(ctx, client, cfg.Namespace(), desired)
	}, decoder.MutateNamespace(cfg.Namespace()))
	if err != nil {
		var setupErr *testutil.RotationSetupError
		if errors.As(err, &setupErr) {
			return setupErr
		}
		return &testutil.RotationSetupError{Kind: testutil.RotationSetupFixtureDecodeFailed, ResourceKind: "FixtureFile", ResourceName: filename}
	}
	if !seen {
		return &testutil.RotationSetupError{Kind: testutil.RotationSetupFixtureDecodeFailed, ResourceKind: "FixtureFile", ResourceName: filename}
	}
	return nil
}

func applyRotationFixtureDocument(ctx context.Context, client *resources.Resources, namespace string, desired k8s.Object) error {
	gvk := desired.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		return &testutil.RotationSetupError{Kind: testutil.RotationSetupFixtureKindMissing, ResourceName: desired.GetName()}
	}
	key := types.NamespacedName{Namespace: desired.GetNamespace(), Name: desired.GetName()}
	if key.Namespace == "" {
		key.Namespace = namespace
		desired.SetNamespace(key.Namespace)
	}
	desiredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(desired)
	if err != nil {
		return &testutil.RotationSetupError{Kind: testutil.RotationSetupFixtureConversionFailed, ResourceKind: gvk.Kind, ResourceName: key.Name}
	}
	current, exists, err := getRotationFixtureObject(ctx, client, key, gvk)
	if err != nil {
		return testutil.RotationSetupApplyFailure(err, gvk.Kind, key.Name)
	}
	if exists {
		return validateExistingRotationFixture(desired, desiredMap["spec"], current, gvk)
	}
	if err := client.Create(ctx, desired); err != nil && !apierrors.IsAlreadyExists(err) {
		return testutil.RotationSetupApplyFailure(err, gvk.Kind, key.Name)
	}
	current, _, err = getRotationFixtureObject(ctx, client, key, gvk)
	if err != nil {
		return testutil.RotationSetupApplyFailure(err, gvk.Kind, key.Name)
	}
	return validateExistingRotationFixture(desired, desiredMap["spec"], current, gvk)
}

func getRotationFixtureObject(ctx context.Context, client *resources.Resources, key types.NamespacedName, gvk schema.GroupVersionKind) (k8s.Object, bool, error) {
	current, err := client.GetScheme().New(gvk)
	if err != nil {
		return nil, false, &testutil.RotationSetupError{Kind: testutil.RotationSetupFixtureKindMissing, ResourceName: key.Name}
	}
	currentObject, ok := current.(k8s.Object)
	if !ok {
		return nil, false, &testutil.RotationSetupError{Kind: testutil.RotationSetupFixtureKindMissing, ResourceKind: gvk.Kind, ResourceName: key.Name}
	}
	if err := client.Get(ctx, key.Name, key.Namespace, currentObject); apierrors.IsNotFound(err) {
		return currentObject, false, nil
	} else if err != nil {
		return nil, false, err
	}
	return currentObject, true, nil
}

func validateExistingRotationFixture(desired k8s.Object, desiredSpec any, current k8s.Object, expected schema.GroupVersionKind) error {
	if !testutil.RotationFixtureIdentityMatches(expected, desired, current) {
		return &testutil.RotationSetupError{Kind: testutil.RotationSetupFixtureIdentityMismatch, ResourceKind: expected.Kind, ResourceName: desired.GetName()}
	}
	currentMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(current)
	if err != nil {
		return &testutil.RotationSetupError{Kind: testutil.RotationSetupFixtureConversionFailed, ResourceKind: expected.Kind, ResourceName: desired.GetName()}
	}
	if !rotationFixtureValueMatches(desiredSpec, currentMap["spec"]) {
		return &testutil.RotationSetupError{Kind: testutil.RotationSetupFixtureSpecMismatch, ResourceKind: expected.Kind, ResourceName: desired.GetName()}
	}
	return nil
}

func rotationFixtureValueMatches(expected, actual any) bool {
	return rotationFixtureMapMatches(expected, actual, true)
}

func rotationFixtureMapMatches(expected, actual any, allowManagedDefaults bool) bool {
	expectedMap, isMap := expected.(map[string]interface{})
	if isMap {
		actualMap, ok := actual.(map[string]interface{})
		if !ok {
			return false
		}
		for key, expectedValue := range expectedMap {
			actualValue, ok := actualMap[key]
			if !ok || !rotationFixtureMapMatches(expectedValue, actualValue, false) {
				return false
			}
		}
		for key := range actualMap {
			if _, expected := expectedMap[key]; expected {
				continue
			}
			if !allowManagedDefaults || !rotationFixtureDefaultedSpecField(key) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(expected, actual)
}

func rotationFixtureDefaultedSpecField(field string) bool {
	switch field {
	case "managementPolicies", "providerConfigRef", "deletionPolicy":
		return true
	default:
		return false
	}
}

func logRotationSetupDiagnostic(t *testing.T, ctx context.Context, client *resources.Resources, phase, bindingName, instanceName, orgName, spaceName string) {
	t.Helper()
	// Reads below intentionally log only identity-presence and condition-state
	// booleans. No raw messages, parameter objects, errors, or Secret data escape.
	binding := &v1alpha1.ServiceCredentialBinding{}
	instance := &v1alpha1.ServiceInstance{}
	org := &v1alpha1.Organization{}
	space := &v1alpha1.Space{}
	bindingRead := client.Get(ctx, runScopedName(bindingName), "", binding) == nil
	instanceRead := client.Get(ctx, runScopedName(instanceName), "", instance) == nil
	orgRead := client.Get(ctx, runScopedName(orgName), "", org) == nil
	spaceRead := client.Get(ctx, runScopedName(spaceName), "", space) == nil
	bindingPaused, bindingExhausted := rotationPauseMarkers(binding.GetAnnotations())
	instancePaused, instanceExhausted := rotationPauseMarkers(instance.GetAnnotations())
	bindingOperationType, bindingOperationState := safeRotationOperation(binding.Status.AtProvider.LastOperation)
	instanceOperationType, instanceOperationState := safeRotationOperation(&instance.Status.AtProvider.LastOperation)
	t.Logf("rotation setup diagnostic phase=%s bindingRead=%t bindingGUIDPresent=%t bindingOperationType=%s bindingOperationState=%s bindingReady=%s bindingSynced=%s bindingPaused=%t bindingCreateExhausted=%t instanceRead=%t instanceGUIDPresent=%t instanceOperationType=%s instanceOperationState=%s instanceReady=%s instanceSynced=%s instancePaused=%t instanceCreateExhausted=%t orgRead=%t orgIDPresent=%t orgReady=%s orgSynced=%s spaceRead=%t spaceIDPresent=%t spaceReady=%s spaceSynced=%s",
		phase, bindingRead, binding.Status.AtProvider.GUID != "", bindingOperationType, bindingOperationState, safeRotationCondition(binding, "Ready"), safeRotationCondition(binding, "Synced"), bindingPaused, bindingExhausted,
		instanceRead, instance.Status.AtProvider.ID != nil && *instance.Status.AtProvider.ID != "", instanceOperationType, instanceOperationState, safeRotationCondition(instance, "Ready"), safeRotationCondition(instance, "Synced"), instancePaused, instanceExhausted,
		orgRead, org.Status.AtProvider.ID != nil && *org.Status.AtProvider.ID != "", safeRotationCondition(org, "Ready"), safeRotationCondition(org, "Synced"), spaceRead, space.Status.AtProvider.ID != "", safeRotationCondition(space, "Ready"), safeRotationCondition(space, "Synced"))
}

func safeRotationOperation(operation *v1alpha1.LastOperation) (string, string) {
	if operation == nil {
		return "none", "none"
	}
	kind := "other"
	switch operation.Type {
	case v1alpha1.LastOperationCreate, v1alpha1.LastOperationUpdate, v1alpha1.LastOperationDelete, v1alpha1.LastOperationInitial:
		kind = operation.Type
	}
	state := "other"
	switch operation.State {
	case v1alpha1.LastOperationSucceeded, v1alpha1.LastOperationFailed, v1alpha1.LastOperationInProgress, "":
		state = operation.State
	}
	if state == "" {
		state = "none"
	}
	return kind, state
}

func safeRotationCondition(object interface {
	GetCondition(xpv2.ConditionType) xpv2.Condition
}, conditionType xpv2.ConditionType) string {
	status := object.GetCondition(conditionType).Status
	switch status {
	case corev1.ConditionTrue, corev1.ConditionFalse, corev1.ConditionUnknown:
		return string(status)
	default:
		return "unknown"
	}
}

func rotationPauseMarkers(annotations map[string]string) (paused, exhausted bool) {
	return annotations["crossplane.io/paused"] == "true", annotations["servicecredentialbinding.cloudfoundry.crossplane.io/max-retry-exceeded"] != ""
}
