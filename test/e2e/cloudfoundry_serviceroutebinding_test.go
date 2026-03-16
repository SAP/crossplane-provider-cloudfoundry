//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	meta "github.com/SAP/crossplane-provider-cloudfoundry/apis"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	resources "sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"

	v1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

func TestCloudFoundryServiceRouteBinding(t *testing.T) {
	var dir = "./crs/serviceRouteBinding"
	var namespace = "service-test"
	var feats = map[string]struct {
		// name of the managed resource
		name string
		// managed resource
		obj k8s.Object
		// updated checks if resource is updated, normally by observing a new value on managed field.
		updated func(k8s.Object) (bool, error)
	}{
		"org":                          {name: "e2e-serviceroutebinding-org", obj: &v1alpha1.Organization{}},
		"space":                        {name: "e2e-serviceroutebinding-space", obj: &v1alpha1.Space{}},
		"domain":                       {name: "e2e-serviceroutebinding-domain", obj: &v1alpha1.Domain{}},
		"serviceinstance":              {name: "e2e-serviceroutebinding-serviceinstance", obj: &v1alpha1.ServiceInstance{}},
		"route":                        {name: "e2e-serviceroutebinding-route", obj: &v1alpha1.Route{}},
		"service_route_binding":        {name: "e2e-serviceroutebinding-binding", obj: &v1alpha1.ServiceRouteBinding{}},
		"service_route_binding_update": {name: "e2e-serviceroutebinding-binding", obj: &v1alpha1.ServiceRouteBinding{}},
	}

	var feat = features.New("ServiceRouteBinding e2e test").Setup(
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			envfuncs.CreateNamespace(namespace)
			cfg.WithNamespace(namespace)

			if err := ApplyResources(ctx, cfg, dir); err != nil {
				t.Fatal("error applying resources", err)
			}
			return ctx
		},
	).Teardown(
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// ignore errors as most, if not all, resources should be deleted in the deletion tests.
			_ = UnapplyResources(ctx, cfg, dir)

			resetTestOrg(ctx, t)

			return ctx
		},
	)

	var steps = [...]string{"org", "space", "domain", "serviceinstance", "route", "service_route_binding"}
	for _, name := range steps {
		ft, ok := feats[name]
		if !ok {
			continue
		}
		feat.Assess(name+":"+ft.name+" observed",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				cr := cfg.Client().Resources()
				if err := cr.Get(ctx, ft.name, cfg.Namespace(), ft.obj); err != nil {
					t.Errorf("error observing resource %s: %s", ft.obj.GetName(), err.Error())
				}
				return ctx
			}).Assess(name+":"+ft.name+" ready",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				ft.obj.SetName(ft.name)
				ft.obj.SetNamespace(cfg.Namespace())
				t.Logf("wait for resource %s to be ready", ft.obj.GetName())
				if err := wait.For(ResourceReady(cfg, ft.obj), wait.WithTimeout(10*time.Minute)); err != nil {
					t.Errorf("error waiting for resource %s to be ready: %s", ft.obj.GetName(), err.Error())
				}
				return ctx
			})
	}

	// Test metadata update by applying updated manifest
	feat.Assess("service_route_binding:e2e-serviceroutebinding-binding apply metadata update",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			r, _ := resources.New(cfg.Client().RESTConfig())
			_ = meta.AddToScheme(r.GetScheme())
			r.WithNamespace(cfg.Namespace())

			err := decoder.DecodeEachFile(
				ctx, os.DirFS(dir), "serviceroutebinding-updated.yaml",
				decoder.CreateIgnoreAlreadyExists(r),
				decoder.MutateNamespace(cfg.Namespace()),
			)
			if err != nil {
				t.Errorf("error applying updated manifest: %s", err.Error())
			}
			t.Logf("Applied updated metadata manifest")
			return ctx
		}).Assess("service_route_binding:e2e-serviceroutebinding-binding ready after metadata update",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			binding := &v1alpha1.ServiceRouteBinding{}
			binding.SetName("e2e-serviceroutebinding-binding")
			binding.SetNamespace(cfg.Namespace())
			t.Logf("Waiting for resource to be ready after metadata update")
			if err := wait.For(ResourceReady(cfg, binding), wait.WithTimeout(10*time.Minute)); err != nil {
				t.Errorf("error waiting for resource to be ready after update: %s", err.Error())
			}
			return ctx
		})

	// deletion assess steps in reversed order as creation assess steps.
	for i := len(steps) - 1; i >= 0; i-- {
		var name = steps[i]
		ft, ok := feats[name]
		if !ok {
			continue
		}
		feat.Assess(name+":"+ft.name+" deleted",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				ft.obj.SetName(ft.name)
				ft.obj.SetNamespace(cfg.Namespace())

				cr := cfg.Client().Resources()
				if err := cr.Delete(ctx, ft.obj); err != nil {
					t.Errorf("error deleting resource %s: %s", ft.obj.GetName(), err.Error())
				}
				if err := wait.For(ResourceDeleted(cfg, ft.obj), wait.WithTimeout(10*time.Minute)); err != nil {
					t.Errorf("error waiting for resource %s to be deleted: %s", ft.obj.GetName(), err.Error())
				}
				return ctx
			},
		)
	}

	testenv.Test(t, feat.Feature())
}
