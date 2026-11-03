//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis"
	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	scbclient "github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/servicecredentialbinding"
	xpresource "github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/retry"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
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
	ctx, cancel := context.WithTimeout(parent, 20*time.Minute)
	defer cancel()
	fixtureDir := crsDir("servicecredentialbinding-rotation/no-ttl")
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 8*time.Minute)
		defer cleanupCancel()
		if err := UnapplyResources(cleanupCtx, cfg, fixtureDir); err != nil {
			t.Errorf("remove rotation fixture: %v", err)
		}
	}()
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
	secret := &corev1.Secret{}
	if err := cfg.Client().Resources().Get(ctx, runScopedName("scb-rotation-no-ttl-secret"), "default", secret); err != nil {
		t.Fatalf("read initial connection Secret: %v", err)
	}
	if len(secret.Data) == 0 {
		t.Fatal("initial connection Secret has no credential data")
	}

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
	ctx, cancel := context.WithTimeout(parent, 20*time.Minute)
	defer cancel()
	fixtureDir := crsDir("servicecredentialbinding-rotation/explicit-ttl")
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 8*time.Minute)
		defer cleanupCancel()
		if err := UnapplyResources(cleanupCtx, cfg, fixtureDir); err != nil {
			t.Errorf("remove cleanup fixture: %v", err)
		}
	}()
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
	if len(actual.OwnerReferences) != 1 || len(expected.OwnerReferences) != 1 {
		return false
	}
	a, e := actual.OwnerReferences[0], expected.OwnerReferences[0]
	return a.APIVersion == e.APIVersion && a.Kind == e.Kind && a.Name == e.Name && a.UID == e.UID && a.Controller != nil && *a.Controller && e.Controller != nil && *e.Controller
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
	current := &v1alpha1.ServiceCredentialBinding{}
	if err := cfg.Client().Resources().Get(ctx, mg.Name, "", current); err != nil {
		t.Fatalf("read current binding for credential-format assertion: %v", err)
	}
	cfBinding := rotationCFClient(t, ctx, cfg, current)
	expected, err := scbclient.GetConnectionDetails(ctx, cfBinding, guid, current.Spec.ConnectionDetailsAsJSON)
	if err != nil {
		t.Fatalf("read replacement credential details: %v", err)
	}
	secret := &corev1.Secret{}
	if err := cfg.Client().Resources().Get(ctx, secretName, "default", secret); err != nil {
		t.Fatalf("read published credential Secret: %v", err)
	}
	if len(expected) == 0 || len(secret.Data) == 0 {
		t.Fatal("replacement credentials or published Secret are empty")
	}
	for key, value := range expected {
		actual, ok := secret.Data[key]
		if !ok || string(actual) != string(value) {
			t.Fatal("published Secret is missing an expected credential entry")
		}
	}
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
	if err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		current := &v1alpha1.ServiceCredentialBinding{}
		if err := cfg.Client().Resources().Get(ctx, name, "", current); err != nil {
			return false, err
		}
		cfBinding := rotationCFClient(t, ctx, cfg, current)
		expected, err := scbclient.GetConnectionDetails(ctx, cfBinding, guid, current.Spec.ConnectionDetailsAsJSON)
		if err != nil {
			return false, nil
		}
		secret := &corev1.Secret{}
		if err := cfg.Client().Resources().Get(ctx, secretName, "default", secret); err != nil {
			return false, nil
		}
		if requireJSONKey && !hasCredentialKey(secret, "credentials") {
			return false, nil
		}
		for key, value := range expected {
			actual, ok := secret.Data[key]
			if !ok || string(actual) != string(value) {
				return false, nil
			}
		}
		return len(expected) != 0, nil
	}); err != nil {
		t.Fatal("timed out waiting for expected published credential entries and retained old-format key")
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

func copyStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in)+1)
	for key, value := range in {
		out[key] = value
	}
	return out
}
