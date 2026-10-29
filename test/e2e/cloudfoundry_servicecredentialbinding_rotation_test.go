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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
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
	if err := ApplyResources(ctx, cfg, fixtureDir); err != nil {
		t.Fatalf("apply rotation fixture: %v", err)
	}
	defer func() {
		if err := UnapplyResources(context.Background(), cfg, fixtureDir); err != nil {
			t.Errorf("remove rotation fixture: %v", err)
		}
	}()

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
	binding.Annotations = copyStringMap(binding.Annotations)
	binding.Annotations[forceRotationAnnotation] = k0
	if err := resourceClient.Update(ctx, binding); err != nil {
		t.Fatalf("set operational force-rotation target: %v", err)
	}

	k1 := waitForRotatedBinding(t, ctx, cfg, name, k0, 5*time.Minute)
	assertPublishedCredentials(t, ctx, cfg, binding, k1, runScopedName("scb-rotation-no-ttl-secret"))
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
		current.Annotations = copyStringMap(current.Annotations)
		current.Annotations[forceRotationAnnotation] = k0
		if err := resourceClient.Update(ctx, current); err != nil {
			t.Fatalf("restore stale force target: %v", err)
		}
		time.Sleep(10 * time.Second)
		assertCurrentGUID(t, ctx, cfg, name, k1)
	}

	// A request targeting the then-current generation remains a valid new action.
	current := &v1alpha1.ServiceCredentialBinding{}
	if err := resourceClient.Get(ctx, name, "", current); err != nil {
		t.Fatalf("read current binding before second rotation: %v", err)
	}
	current.Annotations = copyStringMap(current.Annotations)
	current.Annotations[forceRotationAnnotation] = k1
	if err := resourceClient.Update(ctx, current); err != nil {
		t.Fatalf("target current generation: %v", err)
	}
	k2 := waitForRotatedBinding(t, ctx, cfg, name, k1, 5*time.Minute)
	assertPublishedCredentials(t, ctx, cfg, binding, k2, runScopedName("scb-rotation-no-ttl-secret"))
	assertCurrentGUID(t, ctx, cfg, name, k2)
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
	if err := ApplyResources(ctx, cfg, fixtureDir); err != nil {
		t.Fatalf("apply cleanup fixture: %v", err)
	}
	defer func() {
		if err := UnapplyResources(context.Background(), cfg, fixtureDir); err != nil {
			t.Errorf("remove cleanup fixture: %v", err)
		}
	}()

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
	binding.Annotations = copyStringMap(binding.Annotations)
	binding.Annotations[forceRotationAnnotation] = k0
	if err := resourceClient.Update(ctx, binding); err != nil {
		t.Fatalf("set cleanup rotation target: %v", err)
	}
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
	binding.Spec.ForProvider.Rotation = nil
	binding.Annotations = copyStringMap(binding.Annotations)
	binding.Annotations[forceRotationAnnotation] = k0
	if err := resourceClient.Update(ctx, binding); err != nil {
		t.Fatalf("disable scheduled rotation and restore stale target: %v", err)
	}
	for i := 0; i < 6; i++ {
		time.Sleep(10 * time.Second)
		assertCurrentGUID(t, ctx, cfg, name, activeGUID)
	}
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
	cfBinding := rotationCFClient(t, ctx, cfg, mg)
	details, err := cfBinding.GetDetails(ctx, guid)
	if err != nil {
		t.Fatalf("read replacement credential details: %v", err)
	}
	secret := &corev1.Secret{}
	if err := cfg.Client().Resources().Get(ctx, secretName, "default", secret); err != nil {
		t.Fatalf("read published credential Secret: %v", err)
	}
	if len(details.Credentials) == 0 || len(secret.Data) == 0 {
		t.Fatal("replacement credentials or published Secret are empty")
	}
	want, err := json.Marshal(details.Credentials)
	if err != nil {
		t.Fatalf("marshal replacement credentials for comparison: %v", err)
	}
	got, ok := secret.Data["credentials"]
	if !ok || string(got) != string(want) {
		t.Fatal("published Secret does not match replacement credentials")
	}
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
