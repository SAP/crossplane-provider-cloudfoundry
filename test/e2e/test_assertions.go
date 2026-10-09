//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"

	meta "github.com/SAP/crossplane-provider-cloudfoundry/apis"
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	v1 "k8s.io/api/core/v1"
	wait2 "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	resources "sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// ApplyResources creates resources by applying yaml files in the provided directory.
func ApplyResources(ctx context.Context, cfg *envconf.Config, dir string) error {
	r, _ := resources.New(cfg.Client().RESTConfig())

	// Add custom resource objects so that we can query them via the client
	_ = meta.AddToScheme(r.GetScheme())
	r.WithNamespace(cfg.Namespace())

	// managed resources are cluster scoped, so if we patched them with the test namespace it won't do anything
	return decoder.DecodeEachFile(
		ctx, os.DirFS(dir), "*.yaml",
		decoder.CreateIgnoreAlreadyExists(r),
		decoder.MutateNamespace(cfg.Namespace()),
	)
}

// UnapplyResources delete resources by looping through files in the provided directory.
func UnapplyResources(ctx context.Context, cfg *envconf.Config, dir string) error {
	r, _ := resources.New(cfg.Client().RESTConfig())

	// Add custom resource objects so that we can query them via the client
	_ = meta.AddToScheme(r.GetScheme())
	r.WithNamespace(cfg.Namespace())

	return decoder.DecodeEachFile(
		ctx, os.DirFS(dir), "*.yaml",
		decoder.DeleteHandler(r),
	)
}

// ResourceReady ConditionFunc returns true when the resource is ready to use
func ResourceReady(cfg *envconf.Config, object k8s.Object) wait2.ConditionWithContextFunc {
	var cr = cfg.Client().Resources()
	return conditions.New(cr).ResourceMatch(object, func(object k8s.Object) bool {
		mg := object.(resource.Managed)
		klog.V(4).Infof("Waiting %s to become ready...", mg.GetName())
		condition := mg.GetCondition(xpv1.TypeReady)
		result := condition.Status == v1.ConditionTrue
		klog.V(4).Infof(
			"%s ready status is %v",
			mg.GetName(),
			condition.Status,
		)
		return result
	})
}

func ResourceDeleted(cfg *envconf.Config, object k8s.Object) wait2.ConditionWithContextFunc {
	var cr = cfg.Client().Resources()
	return conditions.New(cr).ResourceDeleted(object)
}

// AssertDefaultAnnotations checks that the Crossplane default tags are set as
// annotations with the expected values and are no longer present as labels.
func AssertDefaultAnnotations(observedLabels, observedAnnotations map[string]*string, crName, expectedKind, providerConfigName string) error {
	if err := assertObservedValue(observedAnnotations, resource.ExternalResourceTagKeyKind, expectedKind, crName); err != nil {
		return fmt.Errorf("default annotation check failed: %w", err)
	}
	if err := assertObservedValue(observedAnnotations, resource.ExternalResourceTagKeyName, crName, crName); err != nil {
		return fmt.Errorf("default annotation check failed: %w", err)
	}
	if providerConfigName != "" {
		if err := assertObservedValue(observedAnnotations, resource.ExternalResourceTagKeyProvider, providerConfigName, crName); err != nil {
			return fmt.Errorf("default annotation check failed: %w", err)
		}
	}
	for _, k := range []string{resource.ExternalResourceTagKeyKind, resource.ExternalResourceTagKeyName, resource.ExternalResourceTagKeyProvider} {
		if _, ok := observedLabels[k]; ok {
			return fmt.Errorf("resource %s still has legacy default label %q", crName, k)
		}
	}
	return nil
}

// assertObservedValue checks that observed[key] exists and equals value.
func assertObservedValue(observed map[string]*string, key, value, crName string) error {
	if observed == nil {
		return fmt.Errorf("observed map is nil for resource %s", crName)
	}
	val, exists := observed[key]
	if !exists {
		return fmt.Errorf("resource %s missing key %q", crName, key)
	}
	if val == nil || *val != value {
		actual := "<nil>"
		if val != nil {
			actual = *val
		}
		return fmt.Errorf("resource %s key %q: expected %q, got %q", crName, key, value, actual)
	}
	return nil
}

// AssertLabelsAndAnnotations checks both user-provided labels and default Crossplane annotations,
// plus user-provided annotations on an eligible CF resource.
// expectedLabels/expectedAnnotations are the user-provided key-value pairs expected in observation.
// expectedKind is the lowercase GVK string like "space.cloudfoundry.crossplane.io".
func AssertLabelsAndAnnotations(
	observedLabels map[string]*string,
	observedAnnotations map[string]*string,
	expectedLabels map[string]string,
	expectedAnnotations map[string]string,
	crName, expectedKind, providerConfigName string,
) error {
	// Check default Crossplane annotations and absence of legacy labels
	if err := AssertDefaultAnnotations(observedLabels, observedAnnotations, crName, expectedKind, providerConfigName); err != nil {
		return err
	}
	// Check user-provided labels
	for k, v := range expectedLabels {
		if err := assertObservedValue(observedLabels, k, v, crName); err != nil {
			return fmt.Errorf("user label check failed: %w", err)
		}
	}
	// Check user-provided annotations
	for k, v := range expectedAnnotations {
		if err := assertObservedValue(observedAnnotations, k, v, crName); err != nil {
			return fmt.Errorf("user annotation check failed: %w", err)
		}
	}
	return nil
}
