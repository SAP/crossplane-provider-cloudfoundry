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

// AssertDefaultLabels checks that the observed labels on a CF resource contain
// the three Crossplane default labels (crossplane-kind, crossplane-name,
// crossplane-providerconfig) with the expected values.
func AssertDefaultLabels(observedLabels map[string]*string, crName, expectedKind, providerConfigName string) error {
	if observedLabels == nil {
		return fmt.Errorf("observed labels map is nil for resource %s", crName)
	}

	type check struct {
		key   string
		value string
	}
	checks := []check{
		{key: "crossplane-kind", value: expectedKind},
		{key: "crossplane-name", value: crName},
	}
	if providerConfigName != "" {
		checks = append(checks, check{key: "crossplane-providerconfig", value: providerConfigName})
	}

	for _, c := range checks {
		val, exists := observedLabels[c.key]
		if !exists {
			return fmt.Errorf("resource %s missing default label %q", crName, c.key)
		}
		if val == nil || *val != c.value {
			actual := "<nil>"
			if val != nil {
				actual = *val
			}
			return fmt.Errorf("resource %s label %q: expected %q, got %q", crName, c.key, c.value, actual)
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

// AssertLabelsAndAnnotations checks both user-provided and default Crossplane labels,
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
	// Check default Crossplane labels
	if err := AssertDefaultLabels(observedLabels, crName, expectedKind, providerConfigName); err != nil {
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
