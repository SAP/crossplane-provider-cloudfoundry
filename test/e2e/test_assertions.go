//go:build e2e

package e2e

import (
	"context"
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

// ApplyResources delete resources by looping through files in the provided directory.
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
