//go:build upgrade

package upgrade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis"
	v1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	scbclient "github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/servicecredentialbinding"
	cfclient "github.com/cloudfoundry/go-cfclient/v3/client"
	cfresource "github.com/cloudfoundry/go-cfclient/v3/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	corev1 "k8s.io/api/core/v1"
	kubeErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

const (
	rotationMigrationBindingName  = "upgrade-test-rotation-service-credential-binding"
	rotationMigrationFailureEnv   = "UPGRADE_TEST_INJECT_ROTATION_MIGRATION_FAILURE"
	rotationMigrationFailurePoint = "after-upgrade-before-post-assessment"
)

var rotationMigrationResourceDirectories = []string{
	"./testdata/customCrs/serviceCredentialBindingRotation/import",
	"./testdata/customCrs/serviceCredentialBindingRotation/serviceInstance",
	"./testdata/customCrs/serviceCredentialBindingRotation/serviceCredentialBinding",
}

type rotationMigrationContextKey struct{}

type rotationMigrationState struct {
	OldGUID        string
	ActiveGUID     string
	KnownGUIDs     map[string]struct{}
	OrgGUID        string
	SpaceGUID      string
	Kube           ctrlclient.Client
	CF             scbclient.ServiceCredentialBinding
	CFAPI          *cfclient.Client
	AncillaryOwner metav1.OwnerReference
}

// Test_ServiceCredentialBinding_Rotation_Migration exercises a legacy retired key
// through the v1.3.0-to-local CRD/controller upgrade. The old controller is paused
// with its legacy empty annotation so the new controller must ignore it and migrate
// the retired-key deadline without rotating the active binding.
func Test_ServiceCredentialBinding_Rotation_Migration(t *testing.T) {
	if fromTag != "v1.3.0" || toTag != "local" {
		t.Skipf("rotation migration scenario requires v1.3.0 -> local, got %s -> %s", fromTag, toTag)
	}
	cleanupState := &rotationMigrationState{KnownGUIDs: make(map[string]struct{})}
	preUpgrade := func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		return rotationMigrationPreUpgrade(ctx, t, cfg, cleanupState)
	}
	upgradeTest := NewCustomUpgradeTest("service-credential-binding-rotation-migration").
		FromVersion(fromTag).
		ToVersion(toTag).
		WithResourceDirectories(rotationMigrationResourceDirectories).
		WithVerifyTimeout(10*time.Minute).
		WithKeepResourcesPausedAfterUpgrade().
		WithCustomPreUpgradeAssessment(
			"Create and pause a legacy retired-key state",
			preUpgrade,
		).
		WithCustomPostUpgradeAssessment(
			"Verify legacy handover migration",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				if os.Getenv(rotationMigrationFailureEnv) == rotationMigrationFailurePoint {
					t.Errorf("injected rotation-migration interruption at %s", rotationMigrationFailurePoint)
					return ctx
				}
				if value := os.Getenv(rotationMigrationFailureEnv); value != "" {
					t.Errorf("unsupported %s value", rotationMigrationFailureEnv)
					return ctx
				}
				return rotationMigrationPostUpgrade(ctx, t, cfg)
			},
		).
		WithCustomPreCleanup(func(ctx context.Context, cfg *envconf.Config) error {
			return rotationMigrationPrepareCleanup(ctx, cfg, cleanupState)
		}).
		WithCustomPostResourceCleanup(func(ctx context.Context, cfg *envconf.Config) error {
			return rotationMigrationVerifyCleanup(ctx, cleanupState)
		}).
		SkipDefaultResourceVerification()
	testenv.Test(t, upgradeTest.Feature())
}

func rotationMigrationPreUpgrade(ctx context.Context, t *testing.T, cfg *envconf.Config, state *rotationMigrationState) context.Context {
	t.Helper()
	kube := rotationMigrationKubeClient(t, cfg)
	state.Kube = kube
	binding := &v1alpha1.ServiceCredentialBinding{}
	key := ctrlclient.ObjectKey{Name: rotationMigrationBindingName}

	if err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 8*time.Minute, true, func(ctx context.Context) (bool, error) {
		if err := kube.Get(ctx, key, binding); err != nil {
			return false, err
		}
		return rotationMigrationReady(binding), nil
	}); err != nil {
		t.Fatalf("wait for initial key binding readiness: %v", err)
	}

	cfBinding, cfAPI := rotationMigrationCFClient(t, ctx, kube, binding)
	oldGUID := binding.Status.AtProvider.GUID
	state.OldGUID, state.ActiveGUID, state.Kube, state.CF, state.CFAPI = oldGUID, oldGUID, kube, cfBinding, cfAPI
	org := &v1alpha1.Organization{}
	if err := kube.Get(ctx, ctrlclient.ObjectKey{Name: "upgrade-test-rotation-org"}, org); err != nil {
		t.Fatalf("read imported organization identity: %v", err)
	}
	space := &v1alpha1.Space{}
	if err := kube.Get(ctx, ctrlclient.ObjectKey{Name: "upgrade-test-rotation-space"}, space); err != nil {
		t.Fatalf("read imported space identity: %v", err)
	}
	state.OrgGUID, state.SpaceGUID = meta.GetExternalName(org), meta.GetExternalName(space)
	if state.OrgGUID == "" || state.SpaceGUID == "" {
		t.Fatal("imported organization or space has no observed Cloud Foundry identity")
	}
	rotationMigrationRememberGUID(state, oldGUID)
	if err := rotationMigrationAssertPublished(t, ctx, cfg, cfBinding, binding, oldGUID); err != nil {
		t.Fatalf("verify initial binding handover: %v", err)
	}

	// v1.3.0 treats annotation presence as a force request and removes it after
	// Create. Use the documented legacy empty form to create one real retiree.
	rotationMigrationUpdate(t, ctx, kube, func(current *v1alpha1.ServiceCredentialBinding) {
		if current.Annotations == nil {
			current.Annotations = map[string]string{}
		}
		current.Annotations[rotationMigrationForceAnnotation] = ""
	})

	if err := wait.PollUntilContextTimeout(ctx, 10*time.Second, 8*time.Minute, true, func(ctx context.Context) (bool, error) {
		if err := kube.Get(ctx, key, binding); err != nil {
			return false, err
		}
		if !rotationMigrationReady(binding) || binding.Status.AtProvider.GUID == oldGUID {
			return false, nil
		}
		return rotationMigrationHasRetiree(binding, oldGUID), nil
	}); err != nil {
		t.Fatalf("wait for legacy replacement and retirement: %v", err)
	}
	activeGUID := binding.Status.AtProvider.GUID
	state.ActiveGUID = activeGUID
	rotationMigrationRememberGUID(state, activeGUID)
	for _, retired := range binding.Status.AtProvider.RetiredKeys {
		if retired != nil && retired.GUID == oldGUID && retired.DeleteAfter != nil {
			t.Fatal("legacy provider unexpectedly initialized the new fallback deadline")
		}
	}
	if err := rotationMigrationAssertPublished(t, ctx, cfg, cfBinding, binding, activeGUID); err != nil {
		t.Fatalf("verify legacy replacement handover: %v", err)
	}
	// The retry-budget pause below stops Cloud Foundry actions but still returns
	// an empty observation that Crossplane's connection Secret publisher applies.
	// Use the full runtime pause before adding ancillary ownership so publication
	// cannot replace the Secret's owner-reference list with the controller alone.
	rotationMigrationUpdate(t, ctx, kube, func(current *v1alpha1.ServiceCredentialBinding) {
		if current.Annotations == nil {
			current.Annotations = map[string]string{}
		}
		current.Annotations[meta.AnnotationKeyReconciliationPaused] = "true"
	})
	if err := rotationMigrationWaitForPaused(ctx, kube); err != nil {
		t.Fatalf("wait for full runtime pause before adding ancillary Secret owner: %v", err)
	}
	if err := rotationMigrationAddAncillaryOwner(ctx, kube, binding, state); err != nil {
		t.Fatalf("add scenario-owned ancillary Secret owner: %v", err)
	}
	if err := rotationMigrationAssertAncillaryOwner(ctx, kube, binding, state); err != nil {
		t.Fatalf("verify ancillary Secret owner before upgrade: %v", err)
	}
	if _, err := cfBinding.Get(ctx, oldGUID); err != nil {
		t.Fatalf("legacy retired binding is not present before upgrade: %v", err)
	}

	// Pause and reintroduce the old empty request atomically. The legacy controller
	// must not consume it, and the upgraded controller must prove it is inert.
	rotationMigrationUpdate(t, ctx, kube, func(current *v1alpha1.ServiceCredentialBinding) {
		if current.Annotations == nil {
			current.Annotations = map[string]string{}
		}
		current.Annotations[rotationMigrationPauseAnnotation] = time.Now().UTC().Format(time.RFC3339)
		current.Annotations[rotationMigrationForceAnnotation] = ""
	})

	return context.WithValue(ctx, rotationMigrationContextKey{}, *state)
}

func rotationMigrationPostUpgrade(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
	t.Helper()
	state, ok := ctx.Value(rotationMigrationContextKey{}).(rotationMigrationState)
	if !ok {
		t.Fatal("rotation migration pre-upgrade state is missing")
	}
	binding := &v1alpha1.ServiceCredentialBinding{}
	if err := state.Kube.Get(ctx, ctrlclient.ObjectKey{Name: rotationMigrationBindingName}, binding); err != nil {
		t.Fatalf("read paused binding after upgrade: %v", err)
	}
	if err := rotationMigrationAssertIdentity(t, binding, state); err != nil {
		t.Fatal(err)
	}
	if !rotationMigrationReady(binding) {
		t.Fatal("active binding is not successfully observed after provider upgrade")
	}
	if !rotationMigrationHasRetiree(binding, state.OldGUID) {
		t.Fatal("legacy retired key record was not preserved by the upgrade")
	}
	if _, paused := binding.Annotations[meta.AnnotationKeyReconciliationPaused]; !paused {
		t.Fatal("full Crossplane pause was not retained across the provider upgrade")
	}
	if condition := binding.GetCondition(xpv2.TypeSynced); condition.Status != corev1.ConditionFalse || condition.Reason != "ReconcilePaused" {
		t.Fatalf("binding is not fully paused after provider upgrade: Synced=%s/%s", condition.Status, condition.Reason)
	}
	if err := rotationMigrationAssertAncillaryOwner(ctx, state.Kube, binding, &state); err != nil {
		t.Fatalf("ancillary Secret owner was not preserved while fully paused across the upgrade: %v", err)
	}
	if _, paused := binding.Annotations[rotationMigrationPauseAnnotation]; !paused {
		t.Fatal("legacy provider retry-budget pause marker was not preserved by the upgrade")
	}
	if !rotationMigrationCRDHasDeleteAfter(t, ctx, cfg) {
		t.Fatal("upgraded live CRD does not preserve retired-key deleteAfter")
	}
	for _, retired := range binding.Status.AtProvider.RetiredKeys {
		if retired != nil && retired.GUID == state.OldGUID && retired.DeleteAfter != nil {
			t.Fatal("paused upgrade initialized fallback retention before handover confirmation")
		}
	}

	// The upgrade harness kept every fixture paused. Once continuity assertions
	// have passed, resume the imported parents and instance so teardown can
	// finalize them even if a later migration assertion fails. Keep the binding
	// paused until its no-Create policy and retained annotation are updated below.
	if err := rotationMigrationResumeSupportingResources(ctx, state.Kube); err != nil {
		t.Fatalf("resume supporting fixture resources after upgrade assertions: %v", err)
	}

	// Use a supported no-Create policy while retaining Update for lifecycle
	// migration. Unpause in the same update, and keep the legacy empty annotation
	// to prove it remains inert after upgrade.
	rotationMigrationUpdate(t, ctx, state.Kube, func(current *v1alpha1.ServiceCredentialBinding) {
		if current.Annotations == nil {
			current.Annotations = map[string]string{}
		}
		current.Spec.ManagementPolicies = xpv2.ManagementPolicies{
			xpv2.ManagementActionObserve,
			xpv2.ManagementActionUpdate,
		}
		delete(current.Annotations, rotationMigrationPauseAnnotation)
		delete(current.Annotations, meta.AnnotationKeyReconciliationPaused)
		current.Annotations[rotationMigrationForceAnnotation] = ""
	})

	var confirmedAt time.Time
	var deadline time.Time
	if err := wait.PollUntilContextTimeout(ctx, 10*time.Second, 8*time.Minute, true, func(ctx context.Context) (bool, error) {
		if err := state.Kube.Get(ctx, ctrlclient.ObjectKey{Name: rotationMigrationBindingName}, binding); err != nil {
			return false, err
		}
		if binding.Status.AtProvider.GUID != state.ActiveGUID || !rotationMigrationReady(binding) || !rotationMigrationHasRetiree(binding, state.OldGUID) {
			return false, nil
		}
		if err := rotationMigrationAssertPublished(t, ctx, cfg, state.CF, binding, state.ActiveGUID); err != nil {
			return false, nil
		}
		if err := rotationMigrationAssertAncillaryOwner(ctx, state.Kube, binding, &state); err != nil {
			return false, nil
		}
		for _, retired := range binding.Status.AtProvider.RetiredKeys {
			if retired != nil && retired.GUID == state.OldGUID && retired.DeleteAfter != nil {
				confirmedAt = time.Now().UTC()
				deadline = retired.DeleteAfter.Time
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		t.Fatalf("wait for safe legacy deadline initialization: %v", err)
	}
	if deadline.Before(confirmedAt.Add(48*time.Hour-time.Minute)) || deadline.After(confirmedAt.Add(48*time.Hour+time.Minute)) {
		t.Fatal("legacy retired-key deadline is not approximately 48 hours after confirmed handover")
	}
	if policies := binding.Spec.ManagementPolicies; len(policies) != 2 || !slices.Contains(policies, xpv2.ManagementActionObserve) || !slices.Contains(policies, xpv2.ManagementActionUpdate) || slices.Contains(policies, xpv2.ManagementActionCreate) {
		t.Fatal("migration did not retain the supported no-Create policy")
	}
	if len(binding.Status.AtProvider.RetiredKeys) != 1 || binding.Status.AtProvider.RetiredKeys[0] == nil || binding.Status.AtProvider.RetiredKeys[0].GUID != state.OldGUID {
		t.Fatal("migration created or retained an unexpected binding generation")
	}
	if _, err := state.CF.Get(ctx, state.OldGUID); err != nil {
		t.Fatalf("legacy key disappeared before its fallback deadline: %v", err)
	}
	if got := meta.GetExternalName(binding); got != state.ActiveGUID {
		t.Fatal("active external-name changed during legacy migration")
	}
	if got := binding.Annotations[rotationMigrationForceAnnotation]; got != "" {
		t.Fatal("legacy empty force-rotation annotation was not retained")
	}

	// Re-read through multiple provider sync periods: the annotation remains inert
	// and the initialized deadline must not slide.
	for range 6 {
		time.Sleep(10 * time.Second)
		if err := state.Kube.Get(ctx, ctrlclient.ObjectKey{Name: rotationMigrationBindingName}, binding); err != nil {
			t.Fatalf("re-read migrated binding: %v", err)
		}
		if err := rotationMigrationAssertIdentity(t, binding, state); err != nil {
			t.Fatal(err)
		}
		if got := binding.Annotations[rotationMigrationForceAnnotation]; got != "" {
			t.Fatal("legacy empty force-rotation annotation changed during replay")
		}
		if !rotationMigrationHasDeadline(binding, state.OldGUID, deadline) {
			t.Fatal("legacy retired-key deadline slid during reconciliation")
		}
	}

	return ctx
}

func rotationMigrationResumeSupportingResources(ctx context.Context, kube ctrlclient.Client) error {
	return resumePausedFixtureResources(ctx, kube,
		&v1alpha1.Organization{ObjectMeta: metav1.ObjectMeta{Name: "upgrade-test-rotation-org"}},
		&v1alpha1.Space{ObjectMeta: metav1.ObjectMeta{Name: "upgrade-test-rotation-space"}},
		&v1alpha1.ServiceInstance{ObjectMeta: metav1.ObjectMeta{Name: "upgrade-test-rotation-service-instance"}},
	)
}

// rotationMigrationPrepareCleanup safely requests deletion of this scenario's
// binding, including when an assessment failed and left the old controller paused.
func rotationMigrationPrepareCleanup(ctx context.Context, cfg *envconf.Config, state *rotationMigrationState) error {
	var errs []error
	if state.Kube == nil {
		kube, err := rotationMigrationNewKubeClient(cfg)
		if err != nil {
			return fmt.Errorf("create Kubernetes client for migration cleanup: %w", err)
		}
		state.Kube = kube
	}

	// Resume only this scenario's parent/instance wrappers before deleting the
	// binding. Cleanup also runs after failed post-upgrade assertions, so this
	// must not depend on the normal migration path having reached its resume step.
	if err := rotationMigrationResumeSupportingResources(ctx, state.Kube); err != nil {
		errs = append(errs, fmt.Errorf("resume supporting fixture resources before cleanup: %w", err))
	}
	if err := rotationMigrationRememberImportedIdentities(ctx, state.Kube, state); err != nil {
		return errors.Join(append(errs, err)...)
	}
	key := ctrlclient.ObjectKey{Name: rotationMigrationBindingName}
	current := &v1alpha1.ServiceCredentialBinding{}
	if err := state.Kube.Get(ctx, key, current); err != nil {
		if kubeErrors.IsNotFound(err) {
			return errors.Join(errs...)
		}
		return errors.Join(append(errs, fmt.Errorf("read owned binding before cleanup: %w", err))...)
	}
	rotationMigrationRememberBinding(state, current)
	if state.CF == nil || state.CFAPI == nil {
		cf, cfAPI, err := rotationMigrationNewCFClient(ctx, state.Kube, current)
		if err != nil {
			return errors.Join(append(errs, fmt.Errorf("create Cloud Foundry client for owned cleanup: %w", err))...)
		}
		state.CF, state.CFAPI = cf, cfAPI
	}

	if err := prepareRotationMigrationBindingDeletion(ctx, state.Kube, key, func(binding *v1alpha1.ServiceCredentialBinding) {
		rotationMigrationRememberBinding(state, binding)
	}); err != nil {
		return errors.Join(append(errs, fmt.Errorf("prepare owned binding deletion: %w", err))...)
	}
	return errors.Join(errs...)
}

// rotationMigrationVerifyCleanup runs after fixture deletion but before the
// ProviderConfig/provider teardown. It checks every active/retired generation
// observed for this scenario and reports all failures without hiding later IDs.
func rotationMigrationVerifyCleanup(ctx context.Context, state *rotationMigrationState) error {
	if len(state.KnownGUIDs) > 0 {
		if state.CF == nil {
			return fmt.Errorf("cannot verify Cloud Foundry cleanup: no owned binding client was established")
		}
		guids := make([]string, 0, len(state.KnownGUIDs))
		for guid := range state.KnownGUIDs {
			guids = append(guids, guid)
		}
		if err := rotationMigrationWaitForCFBindingsGone(ctx, state.CF, guids); err != nil {
			return fmt.Errorf("owned Cloud Foundry binding cleanup for tracked generations failed: %w", err)
		}
	}
	if state.CFAPI == nil && (state.OrgGUID != "" || state.SpaceGUID != "") {
		return fmt.Errorf("cannot verify observed imported Cloud Foundry resources after cleanup")
	}
	if state.CFAPI != nil && state.OrgGUID != "" {
		if _, err := state.CFAPI.Organizations.Get(ctx, state.OrgGUID); err != nil {
			return fmt.Errorf("imported Cloud Foundry organization is not accessible after cleanup")
		}
	}
	if state.CFAPI != nil && state.SpaceGUID != "" {
		if _, err := state.CFAPI.Spaces.Get(ctx, state.SpaceGUID); err != nil {
			return fmt.Errorf("imported Cloud Foundry space is not accessible after cleanup")
		}
	}
	return nil
}

func rotationMigrationRememberGUID(state *rotationMigrationState, guid string) {
	if guid == "" {
		return
	}
	if state.KnownGUIDs == nil {
		state.KnownGUIDs = make(map[string]struct{})
	}
	state.KnownGUIDs[guid] = struct{}{}
}

func rotationMigrationRememberBinding(state *rotationMigrationState, binding *v1alpha1.ServiceCredentialBinding) {
	rotationMigrationRememberGUID(state, meta.GetExternalName(binding))
	rotationMigrationRememberGUID(state, binding.Status.AtProvider.GUID)
	for _, retired := range binding.Status.AtProvider.RetiredKeys {
		if retired != nil {
			rotationMigrationRememberGUID(state, retired.GUID)
		}
	}
}

func rotationMigrationRememberImportedIdentities(ctx context.Context, kube ctrlclient.Client, state *rotationMigrationState) error {
	org := &v1alpha1.Organization{}
	if err := kube.Get(ctx, ctrlclient.ObjectKey{Name: "upgrade-test-rotation-org"}, org); err == nil {
		state.OrgGUID = meta.GetExternalName(org)
	} else if !kubeErrors.IsNotFound(err) {
		return fmt.Errorf("read imported organization during cleanup: %w", err)
	}
	space := &v1alpha1.Space{}
	if err := kube.Get(ctx, ctrlclient.ObjectKey{Name: "upgrade-test-rotation-space"}, space); err == nil {
		state.SpaceGUID = meta.GetExternalName(space)
	} else if !kubeErrors.IsNotFound(err) {
		return fmt.Errorf("read imported space during cleanup: %w", err)
	}
	return nil
}

func rotationMigrationKubeClient(t *testing.T, cfg *envconf.Config) ctrlclient.Client {
	t.Helper()
	client, err := rotationMigrationNewKubeClient(cfg)
	if err != nil {
		t.Fatalf("create Kubernetes API client: %v", err)
	}
	return client
}

func rotationMigrationNewKubeClient(cfg *envconf.Config) (ctrlclient.Client, error) {
	kubeScheme := runtime.NewScheme()
	if err := apis.AddToScheme(kubeScheme); err != nil {
		return nil, fmt.Errorf("register provider API scheme: %w", err)
	}
	if err := corev1.AddToScheme(kubeScheme); err != nil {
		return nil, fmt.Errorf("register core API scheme: %w", err)
	}
	client, err := ctrlclient.New(cfg.Client().RESTConfig(), ctrlclient.Options{Scheme: kubeScheme})
	if err != nil {
		return nil, err
	}
	return client, nil
}

func rotationMigrationCFClient(t *testing.T, ctx context.Context, kube ctrlclient.Client, binding *v1alpha1.ServiceCredentialBinding) (scbclient.ServiceCredentialBinding, *cfclient.Client) {
	t.Helper()
	cf, cfAPI, err := rotationMigrationNewCFClient(ctx, kube, binding)
	if err != nil {
		t.Fatalf("create Cloud Foundry binding client: %v", err)
	}
	return cf, cfAPI
}

func rotationMigrationNewCFClient(ctx context.Context, kube ctrlclient.Client, binding *v1alpha1.ServiceCredentialBinding) (scbclient.ServiceCredentialBinding, *cfclient.Client, error) {
	cf, err := clients.ClientFnBuilder(ctx, kube)(binding)
	if err != nil {
		return nil, nil, err
	}
	return scbclient.NewClient(cf), cf, nil
}

func rotationMigrationUpdate(t *testing.T, ctx context.Context, kube ctrlclient.Client, mutate func(*v1alpha1.ServiceCredentialBinding)) {
	t.Helper()
	key := ctrlclient.ObjectKey{Name: rotationMigrationBindingName}
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &v1alpha1.ServiceCredentialBinding{}
		if err := kube.Get(ctx, key, current); err != nil {
			return err
		}
		mutate(current)
		return kube.Update(ctx, current)
	}); err != nil {
		t.Fatalf("update migration test binding: %v", err)
	}
}

func rotationMigrationWaitForPaused(ctx context.Context, kube ctrlclient.Client) error {
	key := ctrlclient.ObjectKey{Name: rotationMigrationBindingName}
	return wait.PollUntilContextTimeout(ctx, time.Second, time.Minute, true, func(ctx context.Context) (bool, error) {
		binding := &v1alpha1.ServiceCredentialBinding{}
		if err := kube.Get(ctx, key, binding); err != nil {
			return false, err
		}
		condition := binding.GetCondition(xpv2.TypeSynced)
		return condition.Status == corev1.ConditionFalse && condition.Reason == "ReconcilePaused", nil
	})
}

func rotationMigrationReady(binding *v1alpha1.ServiceCredentialBinding) bool {
	return binding.Status.AtProvider.GUID != "" && binding.Status.AtProvider.LastOperation != nil &&
		binding.Status.AtProvider.LastOperation.State == "succeeded"
}

func rotationMigrationHasRetiree(binding *v1alpha1.ServiceCredentialBinding, guid string) bool {
	for _, retired := range binding.Status.AtProvider.RetiredKeys {
		if retired != nil && retired.GUID == guid {
			return true
		}
	}
	return false
}

func rotationMigrationHasDeadline(binding *v1alpha1.ServiceCredentialBinding, guid string, want time.Time) bool {
	for _, retired := range binding.Status.AtProvider.RetiredKeys {
		if retired != nil && retired.GUID == guid && retired.DeleteAfter != nil {
			return retired.DeleteAfter.Time.Equal(want)
		}
	}
	return false
}

func rotationMigrationAssertIdentity(t *testing.T, binding *v1alpha1.ServiceCredentialBinding, state rotationMigrationState) error {
	t.Helper()
	if got := meta.GetExternalName(binding); got != state.ActiveGUID {
		return fmt.Errorf("active external-name changed during migration")
	}
	if binding.Status.AtProvider.GUID != state.ActiveGUID {
		return fmt.Errorf("active ServiceCredentialBinding GUID changed during migration")
	}
	return nil
}

func rotationMigrationAddAncillaryOwner(ctx context.Context, kube ctrlclient.Client, binding *v1alpha1.ServiceCredentialBinding, state *rotationMigrationState) error {
	instance := &v1alpha1.ServiceInstance{}
	if err := kube.Get(ctx, ctrlclient.ObjectKey{Name: "upgrade-test-rotation-service-instance"}, instance); err != nil {
		return fmt.Errorf("read scenario-owned ServiceInstance identity: %w", err)
	}
	if instance.UID == "" {
		return fmt.Errorf("scenario-owned ServiceInstance has no Kubernetes UID")
	}
	controller := false
	state.AncillaryOwner = metav1.OwnerReference{
		APIVersion: v1alpha1.ServiceInstance_GroupVersionKind.GroupVersion().String(),
		Kind:       v1alpha1.ServiceInstance_GroupVersionKind.Kind,
		Name:       instance.Name,
		UID:        instance.UID,
		Controller: &controller,
	}
	ref := binding.Spec.WriteConnectionSecretToReference
	if ref == nil {
		return fmt.Errorf("migration fixture has no connection Secret reference")
	}
	secret := &corev1.Secret{}
	if err := kube.Get(ctx, ctrlclient.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, secret); err != nil {
		return fmt.Errorf("read published connection Secret: %w", err)
	}
	secret.OwnerReferences = append(secret.OwnerReferences, state.AncillaryOwner)
	return kube.Update(ctx, secret)
}

func rotationMigrationAssertAncillaryOwner(ctx context.Context, kube ctrlclient.Client, binding *v1alpha1.ServiceCredentialBinding, state *rotationMigrationState) error {
	ref := binding.Spec.WriteConnectionSecretToReference
	if ref == nil {
		return fmt.Errorf("migration fixture has no connection Secret reference")
	}
	secret := &corev1.Secret{}
	if err := kube.Get(ctx, ctrlclient.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, secret); err != nil {
		return fmt.Errorf("read connection Secret: %w", err)
	}
	for _, owner := range secret.OwnerReferences {
		if owner.APIVersion == state.AncillaryOwner.APIVersion && owner.Kind == state.AncillaryOwner.Kind && owner.Name == state.AncillaryOwner.Name && owner.UID == state.AncillaryOwner.UID && owner.Controller != nil && !*owner.Controller {
			return nil
		}
	}
	return fmt.Errorf("scenario-owned ancillary ServiceInstance owner reference is missing")
}

func rotationMigrationAssertPublished(t *testing.T, ctx context.Context, cfg *envconf.Config, cf scbclient.ServiceCredentialBinding, binding *v1alpha1.ServiceCredentialBinding, guid string) error {
	t.Helper()
	details, err := cf.GetDetails(ctx, guid)
	if err != nil {
		return fmt.Errorf("read CF credential details: %w", err)
	}
	if details == nil {
		return fmt.Errorf("CF returned no credential details")
	}
	secretRef := binding.Spec.WriteConnectionSecretToReference
	if secretRef == nil {
		return fmt.Errorf("migration fixture has no connection Secret reference")
	}
	secret := &corev1.Secret{}
	if err := cfg.Client().Resources().Get(ctx, secretRef.Name, secretRef.Namespace, secret); err != nil {
		return fmt.Errorf("read connection Secret: %w", err)
	}
	want, err := json.Marshal(details.Credentials)
	if err != nil {
		return fmt.Errorf("encode CF credentials")
	}
	if string(secret.Data["credentials"]) != string(want) {
		return fmt.Errorf("published Secret does not match current CF credentials")
	}
	ownerMatches := false
	for _, owner := range secret.OwnerReferences {
		if owner.UID == binding.UID && owner.Name == binding.Name && owner.Controller != nil && *owner.Controller {
			ownerMatches = true
			break
		}
	}
	if !ownerMatches {
		return fmt.Errorf("published Secret is not controller-owned by the binding")
	}
	return nil
}

func rotationMigrationWaitForCFBindingsGone(ctx context.Context, cf scbclient.ServiceCredentialBinding, guids []string) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, 8*time.Minute, true, func(ctx context.Context) (bool, error) {
		allGone := true
		for _, guid := range guids {
			_, err := cf.Get(ctx, guid)
			if err == nil {
				allGone = false
				continue
			}
			if !cfresource.IsResourceNotFoundError(err) && !cfresource.IsServiceBindingNotFoundError(err) {
				return false, err
			}
		}
		return allGone, nil
	})
}

func rotationMigrationCRDHasDeleteAfter(t *testing.T, ctx context.Context, cfg *envconf.Config) bool {
	t.Helper()
	client, err := dynamic.NewForConfig(cfg.Client().RESTConfig())
	if err != nil {
		t.Fatalf("create dynamic client for live CRD check: %v", err)
	}
	crd, err := client.Resource(schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}).Get(
		ctx, "servicecredentialbindings.cloudfoundry.crossplane.io", metav1.GetOptions{},
	)
	if err != nil {
		t.Fatalf("read installed ServiceCredentialBinding CRD: %v", err)
	}
	versions, found, err := unstructured.NestedSlice(crd.Object, "spec", "versions")
	if err != nil || !found {
		return false
	}
	for _, item := range versions {
		version, ok := item.(map[string]interface{})
		if !ok || version["name"] != "v1alpha1" {
			continue
		}
		_, found, err := unstructured.NestedFieldNoCopy(version, "schema", "openAPIV3Schema", "properties", "status", "properties", "atProvider", "properties", "retiredKeys", "items", "properties", "deleteAfter")
		return err == nil && found
	}
	return false
}
