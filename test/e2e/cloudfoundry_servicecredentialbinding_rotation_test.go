//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis"
	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	scbclient "github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/servicecredentialbinding"
	testutil "github.com/SAP/crossplane-provider-cloudfoundry/test"
	xpresource "github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/retry"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

const (
	rotationAssessmentLimit = 20 * time.Minute
	rotationCleanupReserve  = 8 * time.Minute
	rotationFinishMargin    = 30 * time.Second
)

const forceRotationAnnotation = "servicecredentialbinding.cloudfoundry.crossplane.io/force-rotation"

// TestServiceCredentialBindingForceRotation verifies generation-targeted rotation
// and replay stability against a real CF API and the published connection Secret.
func TestServiceCredentialBindingForceRotation(t *testing.T) {
	feature := features.New("targeted service credential binding rotation").Assess("rotate and replay target GUID", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		runForceRotationE2E(t, ctx, cfg)
		return ctx
	})
	testenv.Test(t, feature.Feature())
}

func runForceRotationE2E(t *testing.T, parent context.Context, cfg *envconf.Config) {
	fixtureDir := crsDir("servicecredentialbinding-rotation/no-ttl")
	cleanup := newRotationCleanupOperations(cfg, "scb-rotation-no-ttl", "rotation-instance", "scb-rotation-no-ttl-secret", "rotation-space", "rotation-org")
	applyStarted := false
	defer func() {
		if !applyStarted {
			return
		}
		if err := testutil.RunServiceCredentialBindingRotationCleanup(parent, 8*time.Minute, 2*time.Second, cleanup); err != nil {
			t.Errorf("rotation fixture cleanup incomplete: %v", err)
		}
	}()
	ctx, cancel := rotationAssessmentContext(t, parent)
	defer cancel()
	applyStarted = true
	if err := ApplyResources(ctx, cfg, fixtureDir); err != nil {
		t.Fatalf("apply rotation fixture: %v", err)
	}

	name := runScopedName("scb-rotation-no-ttl")
	binding := &v1alpha1.ServiceCredentialBinding{}
	resourceClient := cfg.Client().Resources()
	if err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		if err := resourceClient.Get(ctx, name, "", binding); err != nil {
			return false, err
		}
		if binding.Status.AtProvider.GUID == "" || binding.Status.AtProvider.LastOperation == nil || binding.Status.AtProvider.LastOperation.Type != "create" || binding.Status.AtProvider.LastOperation.State != "succeeded" {
			return false, nil
		}
		return true, nil
	}); err != nil {
		t.Fatalf("wait for initial binding operation: %v", err)
	}
	k0 := binding.Status.AtProvider.GUID
	waitForCredentialPublication(t, ctx, cfg, name, k0, runScopedName("scb-rotation-no-ttl-secret"), 3*time.Minute)

	cfBinding := rotationCFClient(t, ctx, cfg, binding)
	if _, err := cfBinding.Get(ctx, k0); err != nil {
		t.Fatalf("read initial Cloud Foundry binding: %v", err)
	}
	setForceTarget(t, ctx, cfg, name, k0)

	k1 := waitForRotatedBinding(t, ctx, cfg, name, k0, 5*time.Minute)
	assertPublishedCredentials(t, ctx, cfg, binding, k1, runScopedName("scb-rotation-no-ttl-secret"))
	k0Deadline := waitForFallbackDeadline(t, ctx, cfg, name, k0, cfBinding)
	assertFallbackDeadlineNear(t, k0Deadline, time.Now())
	if _, err := cfBinding.Get(ctx, k0); err != nil {
		t.Fatalf("retired key %q was not retained during the overlap: %v", k0, err)
	}
	assertForceTarget(t, ctx, cfg, name, k0)

	// Restore the same operational annotation repeatedly, including after fresh
	// reads. A retained stale target must never retire the replacement generation.
	for i := 0; i < 6; i++ {
		if err := ctx.Err(); err != nil {
			t.Fatal(err)
		}
		current := &v1alpha1.ServiceCredentialBinding{}
		if err := resourceClient.Get(ctx, name, "", current); err != nil {
			t.Fatalf("reload binding during replay check: %v", err)
		}
		setForceTarget(t, ctx, cfg, name, k0)
		time.Sleep(10 * time.Second)
		assertCurrentGUID(t, ctx, cfg, name, k1)
	}

	// Switch publication format on the same Secret. The runtime publisher merge-patches
	// data, so the old JSON key remains while the expected flattened entries appear.
	setConnectionDetailsFormat(t, ctx, cfg, name, false)
	waitForPublishedCredentialsWithAdditionalKeys(t, ctx, cfg, name, k1, runScopedName("scb-rotation-no-ttl-secret"), true)

	// A request targeting the then-current generation remains a valid new action.
	current := &v1alpha1.ServiceCredentialBinding{}
	if err := resourceClient.Get(ctx, name, "", current); err != nil {
		t.Fatalf("read current binding before second rotation: %v", err)
	}
	setForceTarget(t, ctx, cfg, name, k1)
	k2 := waitForRotatedBinding(t, ctx, cfg, name, k1, 5*time.Minute)
	assertPublishedCredentials(t, ctx, cfg, binding, k2, runScopedName("scb-rotation-no-ttl-secret"))
	k1Deadline := waitForFallbackDeadline(t, ctx, cfg, name, k1, cfBinding)
	assertFallbackDeadlineNear(t, k1Deadline, time.Now())
	assertFallbackDeadlineUnchanged(t, ctx, cfg, name, k0, k0Deadline)
	assertCurrentGUID(t, ctx, cfg, name, k2)
	for i := 0; i < 6; i++ {
		setForceTarget(t, ctx, cfg, name, k0)
		time.Sleep(10 * time.Second)
		assertCurrentGUID(t, ctx, cfg, name, k2)
		setForceTarget(t, ctx, cfg, name, k1)
		time.Sleep(10 * time.Second)
		assertCurrentGUID(t, ctx, cfg, name, k2)
	}
	assertFallbackDeadlineUnchanged(t, ctx, cfg, name, k0, k0Deadline)
}

// TestServiceCredentialBindingRetiredKeyCleanup exercises real CF deletion with
// a short explicit TTL; the 48-hour fallback is covered by controlled-clock tests.
func TestServiceCredentialBindingRetiredKeyCleanup(t *testing.T) {
	feature := features.New("service credential binding retired-key cleanup").Assess("rotate and await retired key cleanup", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		runRetiredKeyCleanupE2E(t, ctx, cfg)
		return ctx
	})
	testenv.Test(t, feature.Feature())
}

func runRetiredKeyCleanupE2E(t *testing.T, parent context.Context, cfg *envconf.Config) {
	fixtureDir := crsDir("servicecredentialbinding-rotation/explicit-ttl")
	cleanup := newRotationCleanupOperations(cfg, "scb-rotation-cleanup", "cleanup-instance", "scb-rotation-cleanup-secret", "cleanup-space", "cleanup-org")
	applyStarted := false
	defer func() {
		if !applyStarted {
			return
		}
		if err := testutil.RunServiceCredentialBindingRotationCleanup(parent, 8*time.Minute, 2*time.Second, cleanup); err != nil {
			t.Errorf("cleanup fixture teardown incomplete: %v", err)
		}
	}()
	ctx, cancel := rotationAssessmentContext(t, parent)
	defer cancel()
	applyStarted = true
	if err := ApplyResources(ctx, cfg, fixtureDir); err != nil {
		t.Fatalf("apply cleanup fixture: %v", err)
	}

	name := runScopedName("scb-rotation-cleanup")
	binding := &v1alpha1.ServiceCredentialBinding{}
	resourceClient := cfg.Client().Resources()
	if err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		if err := resourceClient.Get(ctx, name, "", binding); err != nil {
			return false, err
		}
		return binding.Status.AtProvider.GUID != "", nil
	}); err != nil {
		t.Fatalf("wait for initial binding: %v", err)
	}
	k0 := binding.Status.AtProvider.GUID
	waitForCredentialPublication(t, ctx, cfg, name, k0, runScopedName("scb-rotation-cleanup-secret"), 3*time.Minute)
	cfBinding := rotationCFClient(t, ctx, cfg, binding)
	if _, err := cfBinding.Get(ctx, k0); err != nil {
		t.Fatalf("read initial Cloud Foundry binding: %v", err)
	}
	if err := resourceClient.Get(ctx, name, "", binding); err != nil {
		t.Fatalf("reload binding before rotation: %v", err)
	}
	setForceTarget(t, ctx, cfg, name, k0)
	k1 := waitForRotatedBinding(t, ctx, cfg, name, k0, 5*time.Minute)
	assertPublishedCredentials(t, ctx, cfg, binding, k1, runScopedName("scb-rotation-cleanup-secret"))
	if err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 8*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := cfBinding.Get(ctx, k0)
		if clients.ErrorIsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	}); err != nil {
		t.Fatalf("wait for retired CF key disappearance: %v", err)
	}
	active := &v1alpha1.ServiceCredentialBinding{}
	if err := resourceClient.Get(ctx, name, "", active); err != nil {
		t.Fatalf("read active binding after retired-key cleanup: %v", err)
	}
	activeGUID := active.Status.AtProvider.GUID
	if activeGUID == "" {
		t.Fatal("active binding GUID is empty after retired-key cleanup")
	}
	if _, err := cfBinding.Get(ctx, activeGUID); err != nil {
		t.Fatalf("active replacement key disappeared: %v", err)
	}

	// Once K0 is actually gone, disable scheduled rotation and replay the stale
	// target. The target remains inert even after its retired-key record is GC'd.
	if err := resourceClient.Get(ctx, name, "", binding); err != nil {
		t.Fatalf("reload binding after retired-key cleanup: %v", err)
	}
	activeGUID = binding.Status.AtProvider.GUID
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &v1alpha1.ServiceCredentialBinding{}
		if err := resourceClient.Get(ctx, name, "", current); err != nil {
			return err
		}
		current.Spec.ForProvider.Rotation = nil
		current.Annotations = copyStringMap(current.Annotations)
		current.Annotations[forceRotationAnnotation] = k0
		return resourceClient.Update(ctx, current)
	}); err != nil {
		t.Fatalf("disable scheduled rotation and restore stale target: %v", err)
	}
	for i := 0; i < 6; i++ {
		time.Sleep(10 * time.Second)
		assertCurrentGUID(t, ctx, cfg, name, activeGUID)
	}
}

func setForceTarget(t *testing.T, ctx context.Context, cfg *envconf.Config, name, target string) {
	t.Helper()
	client := cfg.Client().Resources()
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &v1alpha1.ServiceCredentialBinding{}
		if err := client.Get(ctx, name, "", current); err != nil {
			return err
		}
		current.Annotations = copyStringMap(current.Annotations)
		current.Annotations[forceRotationAnnotation] = target
		return client.Update(ctx, current)
	}); err != nil {
		t.Fatalf("set operational force-rotation target: %v", err)
	}
}

func waitForFallbackDeadline(t *testing.T, ctx context.Context, cfg *envconf.Config, name, retiredGUID string, cfBinding scbclient.ServiceCredentialBinding) time.Time {
	t.Helper()
	var deadline time.Time
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		binding := &v1alpha1.ServiceCredentialBinding{}
		if err := cfg.Client().Resources().Get(ctx, name, "", binding); err != nil {
			return false, err
		}
		for _, retired := range binding.Status.AtProvider.RetiredKeys {
			if retired != nil && retired.GUID == retiredGUID && retired.DeleteAfter != nil {
				deadline = retired.DeleteAfter.Time
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		reportFallbackDeadlineDiagnostics(t, ctx, cfg, name, retiredGUID, cfBinding)
		t.Fatalf("wait for fallback retention deadline of retired key: %v", err)
	}
	return deadline
}

// reportFallbackDeadlineDiagnostics logs only lifecycle booleans and non-secret
// operation metadata. Never emit Secret data, credential details, or raw API errors.
func reportFallbackDeadlineDiagnostics(t *testing.T, ctx context.Context, cfg *envconf.Config, name, retiredGUID string, cfBinding scbclient.ServiceCredentialBinding) {
	t.Helper()
	binding := &v1alpha1.ServiceCredentialBinding{}
	if err := cfg.Client().Resources().Get(ctx, name, "", binding); err != nil {
		t.Logf("handover diagnostics: bindingRead=false")
		return
	}

	retiredFound, deadlinePresent := false, false
	for _, retired := range binding.Status.AtProvider.RetiredKeys {
		if retired != nil && retired.GUID == retiredGUID {
			retiredFound = true
			deadlinePresent = retired.DeleteAfter != nil
			break
		}
	}
	operationType, operationState := "", ""
	if operation := binding.Status.AtProvider.LastOperation; operation != nil {
		operationType, operationState = operation.Type, operation.State
	}
	externalNameMatchesStatus := binding.Annotations["crossplane.io/external-name"] != "" && binding.Annotations["crossplane.io/external-name"] == binding.Status.AtProvider.GUID
	rotationConfigured := binding.Spec.ForProvider.Rotation != nil
	ttlConfigured := rotationConfigured && binding.Spec.ForProvider.Rotation.TTL != nil
	crdSchema := inspectLiveRetiredKeySchema(ctx, cfg)

	secretFound, secretTypeMatches, ownerMatches, credentialDataMatches := false, false, false, false
	ref := binding.Spec.WriteConnectionSecretToReference
	if ref != nil {
		secret := &corev1.Secret{}
		if err := cfg.Client().Resources().Get(ctx, ref.Name, ref.Namespace, secret); err == nil {
			secretFound = true
			expected := xpresource.ConnectionSecretFor(binding, v1alpha1.ServiceCredentialBindingGroupVersionKind)
			secretTypeMatches = secret.Type == xpresource.SecretTypeConnection
			ownerMatches = sameConnectionSecretOwner(secret, expected)
			if cfBinding != nil && binding.Status.AtProvider.GUID != "" {
				if details, err := cfBinding.GetDetails(ctx, binding.Status.AtProvider.GUID); err == nil && details != nil {
					if expectedData, err := json.Marshal(details.Credentials); err == nil {
						actualData, ok := secret.Data["credentials"]
						credentialDataMatches = ok && string(actualData) == string(expectedData)
					}
				}
			}
		}
	}

	t.Logf("handover diagnostics: bindingRead=true activeGUIDPresent=%t externalNameMatchesStatus=%t lastOperationType=%q lastOperationState=%q retiredRecordFound=%t deleteAfterPresent=%t rotationConfigured=%t ttlConfigured=%t liveCRDRead=%t liveCRDVersionFound=%t liveRetiredKeysSchemaFound=%t liveDeleteAfterSchemaFound=%t secretConfigured=%t secretFound=%t secretTypeMatches=%t ownerMatches=%t credentialDataMatches=%t",
		binding.Status.AtProvider.GUID != "", externalNameMatchesStatus, operationType, operationState, retiredFound, deadlinePresent, rotationConfigured, ttlConfigured, crdSchema.read, crdSchema.versionFound, crdSchema.retiredKeysFound, crdSchema.deleteAfterFound, ref != nil, secretFound, secretTypeMatches, ownerMatches, credentialDataMatches)
}

type retiredKeySchemaDiagnostics struct {
	read             bool
	versionFound     bool
	retiredKeysFound bool
	deleteAfterFound bool
}

func inspectLiveRetiredKeySchema(ctx context.Context, cfg *envconf.Config) retiredKeySchemaDiagnostics {
	result := retiredKeySchemaDiagnostics{}
	client, err := dynamic.NewForConfig(cfg.Client().RESTConfig())
	if err != nil {
		return result
	}
	crd, err := client.Resource(schema.GroupVersionResource{
		Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions",
	}).Get(ctx, "servicecredentialbindings.cloudfoundry.crossplane.io", metav1.GetOptions{})
	if err != nil {
		return result
	}
	result.read = true
	versions, found, err := unstructured.NestedSlice(crd.Object, "spec", "versions")
	if err != nil || !found {
		return result
	}
	for _, item := range versions {
		version, ok := item.(map[string]interface{})
		if !ok || version["name"] != "v1alpha1" {
			continue
		}
		result.versionFound = true
		_, result.retiredKeysFound, err = unstructured.NestedFieldNoCopy(version, "schema", "openAPIV3Schema", "properties", "status", "properties", "atProvider", "properties", "retiredKeys", "items", "properties")
		if err != nil {
			result.retiredKeysFound = false
			return result
		}
		_, result.deleteAfterFound, err = unstructured.NestedFieldNoCopy(version, "schema", "openAPIV3Schema", "properties", "status", "properties", "atProvider", "properties", "retiredKeys", "items", "properties", "deleteAfter")
		if err != nil {
			result.deleteAfterFound = false
		}
		return result
	}
	return result
}

func sameConnectionSecretOwner(actual, expected *corev1.Secret) bool {
	if actual == nil || expected == nil {
		return false
	}
	a, e := metav1.GetControllerOf(actual), metav1.GetControllerOf(expected)
	return a != nil && e != nil && a.APIVersion == e.APIVersion && a.Kind == e.Kind && a.Name == e.Name && a.UID == e.UID
}

func assertFallbackDeadlineNear(t *testing.T, deadline, observedAt time.Time) {
	t.Helper()
	if deadline.Before(observedAt.Add(47*time.Hour)) || deadline.After(observedAt.Add(49*time.Hour)) {
		t.Fatalf("fallback deadline is outside the expected handover window (observed %s after handover check)", deadline.Sub(observedAt).Round(time.Minute))
	}
}

func assertFallbackDeadlineUnchanged(t *testing.T, ctx context.Context, cfg *envconf.Config, name, retiredGUID string, expected time.Time) {
	t.Helper()
	binding := &v1alpha1.ServiceCredentialBinding{}
	if err := cfg.Client().Resources().Get(ctx, name, "", binding); err != nil {
		t.Fatalf("read binding to verify stable fallback deadline: %v", err)
	}
	for _, retired := range binding.Status.AtProvider.RetiredKeys {
		if retired != nil && retired.GUID == retiredGUID && retired.DeleteAfter != nil {
			if !retired.DeleteAfter.Time.Equal(expected) {
				t.Fatalf("fallback deadline changed across later rotation or stale replay")
			}
			return
		}
	}
	t.Fatalf("retired key %q or its fallback deadline is missing", retiredGUID)
}

func waitForRotatedBinding(t *testing.T, ctx context.Context, cfg *envconf.Config, name, oldGUID string, timeout time.Duration) string {
	t.Helper()
	var newGUID string
	client := cfg.Client().Resources()
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		binding := &v1alpha1.ServiceCredentialBinding{}
		if err := client.Get(ctx, name, "", binding); err != nil {
			return false, err
		}
		if binding.Status.AtProvider.GUID == "" || binding.Status.AtProvider.GUID == oldGUID {
			return false, nil
		}
		if binding.Status.AtProvider.LastOperation == nil || binding.Status.AtProvider.LastOperation.State != "succeeded" {
			return false, nil
		}
		newGUID = binding.Status.AtProvider.GUID
		return true, nil
	})
	if err != nil {
		t.Fatalf("wait for replacement of %q: %v", oldGUID, err)
	}
	return newGUID
}

func assertCurrentGUID(t *testing.T, ctx context.Context, cfg *envconf.Config, name, expected string) {
	t.Helper()
	binding := &v1alpha1.ServiceCredentialBinding{}
	if err := cfg.Client().Resources().Get(ctx, name, "", binding); err != nil {
		t.Fatalf("read current binding: %v", err)
	}
	if got := binding.Status.AtProvider.GUID; got != expected {
		t.Fatalf("current binding GUID changed during stale-target replay (expected %s)", expected)
	}
}

func assertForceTarget(t *testing.T, ctx context.Context, cfg *envconf.Config, name, expected string) {
	t.Helper()
	binding := &v1alpha1.ServiceCredentialBinding{}
	if err := cfg.Client().Resources().Get(ctx, name, "", binding); err != nil {
		t.Fatalf("read binding annotation: %v", err)
	}
	if got := binding.Annotations[forceRotationAnnotation]; got != expected {
		t.Fatalf("force-rotation annotation changed (expected retained target %s)", expected)
	}
}

func assertPublishedCredentials(t *testing.T, ctx context.Context, cfg *envconf.Config, mg *v1alpha1.ServiceCredentialBinding, guid, secretName string) {
	t.Helper()
	waitForCredentialPublication(t, ctx, cfg, mg.Name, guid, secretName, 3*time.Minute)
}

func setConnectionDetailsFormat(t *testing.T, ctx context.Context, cfg *envconf.Config, name string, asJSON bool) {
	t.Helper()
	client := cfg.Client().Resources()
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &v1alpha1.ServiceCredentialBinding{}
		if err := client.Get(ctx, name, "", current); err != nil {
			return err
		}
		current.Spec.ConnectionDetailsAsJSON = asJSON
		return client.Update(ctx, current)
	}); err != nil {
		t.Fatalf("change connection-detail publication format: %v", err)
	}
}

func waitForPublishedCredentialsWithAdditionalKeys(t *testing.T, ctx context.Context, cfg *envconf.Config, name, guid, secretName string, requireJSONKey bool) {
	t.Helper()
	waitForCredentialPublication(t, ctx, cfg, name, guid, secretName, 3*time.Minute)
	if requireJSONKey {
		secret := &corev1.Secret{}
		if err := cfg.Client().Resources().Get(ctx, secretName, "default", secret); err != nil {
			t.Fatalf("read published credential Secret after handover: %v", err)
		}
		if !hasCredentialKey(secret, "credentials") {
			t.Fatal("previous JSON credential key was not retained during format migration")
		}
	}
}

// waitForCredentialPublication separately confirms the active generation and its
// published Secret. A successful CF operation alone is not publication proof.
func waitForCredentialPublication(t *testing.T, ctx context.Context, cfg *envconf.Config, name, targetGUID, secretName string, timeout time.Duration) {
	t.Helper()
	err := testutil.WaitForServiceCredentialBindingPublication(ctx, timeout, 5*time.Second, targetGUID, true, func(ctx context.Context) (testutil.ServiceCredentialBindingPublicationSnapshot, error) {
		current := &v1alpha1.ServiceCredentialBinding{}
		if err := cfg.Client().Resources().Get(ctx, name, "", current); err != nil {
			return testutil.ServiceCredentialBindingPublicationSnapshot{}, nil
		}
		if current.Status.AtProvider.GUID != targetGUID || current.Status.AtProvider.LastOperation == nil || current.Status.AtProvider.LastOperation.State != v1alpha1.LastOperationSucceeded {
			return testutil.ServiceCredentialBindingPublicationSnapshot{}, nil
		}
		ref := current.Spec.WriteConnectionSecretToReference
		if ref == nil || ref.Name != secretName {
			return testutil.ServiceCredentialBindingPublicationSnapshot{}, nil
		}
		cfBinding := rotationCFClient(t, ctx, cfg, current)
		expected, err := scbclient.GetConnectionDetails(ctx, cfBinding, targetGUID, current.Spec.ConnectionDetailsAsJSON)
		if err != nil {
			return testutil.ServiceCredentialBindingPublicationSnapshot{}, nil
		}
		secret := &corev1.Secret{}
		if err := cfg.Client().Resources().Get(ctx, ref.Name, ref.Namespace, secret); err != nil {
			return testutil.ServiceCredentialBindingPublicationSnapshot{}, nil
		}
		expectedOwner := xpresource.ConnectionSecretFor(current, v1alpha1.ServiceCredentialBindingGroupVersionKind)
		return testutil.ServiceCredentialBindingPublicationSnapshot{
			BindingGUID:     current.Status.AtProvider.GUID,
			SecretNamespace: ref.Namespace,
			SecretName:      ref.Name,
			ExpectedDetails: expected,
			Secret:          secret,
			ExpectedOwner:   expectedOwner,
		}, nil
	})
	if err != nil {
		t.Fatalf("wait for current-generation credential publication: %v", err)
	}
}

func hasCredentialKey(secret *corev1.Secret, key string) bool {
	_, ok := secret.Data[key]
	return ok
}

func rotationCFClient(t *testing.T, ctx context.Context, cfg *envconf.Config, mg *v1alpha1.ServiceCredentialBinding) scbclient.ServiceCredentialBinding {
	t.Helper()
	kubeScheme := scheme.Scheme
	if err := apis.AddToScheme(kubeScheme); err != nil {
		t.Fatalf("register provider scheme: %v", err)
	}
	if err := corev1.AddToScheme(kubeScheme); err != nil {
		t.Fatalf("register core scheme: %v", err)
	}
	kube, err := ctrlclient.New(cfg.Client().RESTConfig(), ctrlclient.Options{Scheme: kubeScheme})
	if err != nil {
		t.Fatalf("create Kubernetes client for CF credentials: %v", err)
	}
	cf, err := clients.ClientFnBuilder(ctx, kube)(mg)
	if err != nil {
		t.Fatalf("create Cloud Foundry client: %v", err)
	}
	return scbclient.NewClient(cf)
}

func rotationAssessmentContext(t *testing.T, parent context.Context) (context.Context, context.CancelFunc) {
	t.Helper()
	limit := rotationAssessmentLimit
	if deadline, ok := t.Deadline(); ok {
		available := time.Until(deadline) - rotationCleanupReserve - rotationFinishMargin
		if available <= 0 {
			t.Fatalf("insufficient test-process time remains for assessment and bounded rotation cleanup")
		}
		if available < limit {
			limit = available
		}
	}
	return context.WithTimeout(parent, limit)
}

func newRotationCleanupOperations(cfg *envconf.Config, bindingName, instanceName, secretName, spaceName, orgName string) testutil.ServiceCredentialBindingRotationCleanup {
	kubeScheme := runtime.NewScheme()
	if err := apis.AddToScheme(kubeScheme); err != nil {
		return failedRotationCleanupOperations()
	}
	if err := corev1.AddToScheme(kubeScheme); err != nil {
		return failedRotationCleanupOperations()
	}
	kube, err := ctrlclient.New(cfg.Client().RESTConfig(), ctrlclient.Options{Scheme: kubeScheme})
	if err != nil {
		return failedRotationCleanupOperations()
	}

	var cfBinding scbclient.ServiceCredentialBinding
	var externalCheckErr error
	bindingKey := types.NamespacedName{Name: runScopedName(bindingName)}
	instanceKey := types.NamespacedName{Name: runScopedName(instanceName)}
	secretKey := types.NamespacedName{Namespace: "default", Name: runScopedName(secretName)}
	spaceKey := types.NamespacedName{Name: runScopedName(spaceName)}
	orgKey := types.NamespacedName{Name: runScopedName(orgName)}

	return testutil.ServiceCredentialBindingRotationCleanup{
		CaptureBindingGUIDs: func(ctx context.Context) ([]string, error) {
			binding := &v1alpha1.ServiceCredentialBinding{}
			if err := kube.Get(ctx, bindingKey, binding); err != nil {
				if apierrors.IsNotFound(err) {
					return nil, nil
				}
				err := errors.New("cannot read the owned binding identity for cleanup")
				externalCheckErr = errors.New("Cloud Foundry binding identity could not be recovered")
				return nil, err
			}
			guids := []string{binding.Annotations["crossplane.io/external-name"], binding.Status.AtProvider.GUID}
			for _, retired := range binding.Status.AtProvider.RetiredKeys {
				if retired != nil {
					guids = append(guids, retired.GUID)
				}
			}
			cf, err := clients.ClientFnBuilder(ctx, kube)(binding)
			if err != nil {
				externalCheckErr = errors.New("Cloud Foundry verification client could not be created")
				return guids, nil
			}
			cfBinding = scbclient.NewClient(cf)
			return guids, nil
		},
		CaptureInstanceGUID: func(ctx context.Context) (string, error) {
			instance := &v1alpha1.ServiceInstance{}
			if err := kube.Get(ctx, instanceKey, instance); err != nil {
				if apierrors.IsNotFound(err) {
					return "", nil
				}
				return "", errors.New("cannot read the owned ServiceInstance identity for cleanup")
			}
			return instance.Annotations["crossplane.io/external-name"], nil
		},
		ExternalCheckError: func() error { return externalCheckErr },
		DeleteBinding: func(ctx context.Context) error {
			return testutil.IgnoreRotationCleanupNotFound(kube.Delete(ctx, &v1alpha1.ServiceCredentialBinding{ObjectMeta: metav1.ObjectMeta{Name: bindingKey.Name}}))
		},
		BindingExists: func(ctx context.Context) (bool, error) {
			return rotationObjectExists(ctx, kube, bindingKey, &v1alpha1.ServiceCredentialBinding{})
		},
		BindingExistsInCF: func(ctx context.Context, guid string) (bool, error) {
			if cfBinding == nil {
				return false, errors.New("Cloud Foundry verification client is unavailable")
			}
			present, err := testutil.ServiceCredentialBindingExistsInCF(ctx, cfBinding, guid)
			if err != nil {
				return true, errors.New("Cloud Foundry binding verification request failed")
			}
			return present, nil
		},
		DeleteSecret: func(ctx context.Context) error {
			return testutil.IgnoreRotationCleanupNotFound(kube.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: secretKey.Namespace, Name: secretKey.Name}}))
		},
		SecretExists: func(ctx context.Context) (bool, error) {
			return rotationObjectExists(ctx, kube, secretKey, &corev1.Secret{})
		},
		DeleteInstance: func(ctx context.Context) error {
			return testutil.IgnoreRotationCleanupNotFound(kube.Delete(ctx, &v1alpha1.ServiceInstance{ObjectMeta: metav1.ObjectMeta{Name: instanceKey.Name}}))
		},
		InstanceExists: func(ctx context.Context) (bool, error) {
			return rotationObjectExists(ctx, kube, instanceKey, &v1alpha1.ServiceInstance{})
		},
		DeleteImports: func(ctx context.Context) error {
			spaceErr := testutil.IgnoreRotationCleanupNotFound(kube.Delete(ctx, &v1alpha1.Space{ObjectMeta: metav1.ObjectMeta{Name: spaceKey.Name}}))
			orgErr := testutil.IgnoreRotationCleanupNotFound(kube.Delete(ctx, &v1alpha1.Organization{ObjectMeta: metav1.ObjectMeta{Name: orgKey.Name}}))
			return errors.Join(spaceErr, orgErr)
		},
		ImportsExist: func(ctx context.Context) (bool, error) {
			spaceExists, err := rotationObjectExists(ctx, kube, spaceKey, &v1alpha1.Space{})
			if err != nil || spaceExists {
				return spaceExists, err
			}
			return rotationObjectExists(ctx, kube, orgKey, &v1alpha1.Organization{})
		},
	}
}

func failedRotationCleanupOperations() testutil.ServiceCredentialBindingRotationCleanup {
	cleanupErr := errors.New("Kubernetes cleanup client initialization failed")
	fail := func(context.Context) error { return cleanupErr }
	return testutil.ServiceCredentialBindingRotationCleanup{
		CaptureBindingGUIDs: func(context.Context) ([]string, error) { return nil, cleanupErr },
		CaptureInstanceGUID: func(context.Context) (string, error) { return "", cleanupErr },
		ExternalCheckError:  func() error { return cleanupErr },
		DeleteBinding:       fail,
		BindingExists:       func(context.Context) (bool, error) { return true, cleanupErr },
		DeleteSecret:        fail,
		SecretExists:        func(context.Context) (bool, error) { return true, cleanupErr },
		DeleteInstance:      fail,
		InstanceExists:      func(context.Context) (bool, error) { return true, cleanupErr },
		DeleteImports:       fail,
		ImportsExist:        func(context.Context) (bool, error) { return true, cleanupErr },
	}
}

func rotationObjectExists(ctx context.Context, kube ctrlclient.Client, key types.NamespacedName, object ctrlclient.Object) (bool, error) {
	if err := kube.Get(ctx, key, object); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return true, errors.New("Kubernetes resource existence check failed")
	}
	return true, nil
}

func copyStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in)+1)
	for key, value := range in {
		out[key] = value
	}
	return out
}
