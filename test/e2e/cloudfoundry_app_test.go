//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"slices"
	"testing"
	"time"

	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

func TestCloudfoundry_App(t *testing.T) {
	var dir = "./crs/app"
	var namespace = "app-test"
	var feats = map[string]struct {
		// name of the managed resource
		name string
		// managed resource
		obj k8s.Object
		// updated checks if resource is updated, normally by observing a new value on managed field.
		updated func(k8s.Object) (bool, error)
	}{
		"space":               {name: "app-space", obj: &v1alpha1.Space{}},
		"domain":              {name: "app-domain", obj: &v1alpha1.Domain{}},
		"route":               {name: "app-route-domainref", obj: &v1alpha1.Route{}},
		"app":                 {name: "e2e-app", obj: &v1alpha1.App{}},
		"route-2":             {name: "app-route-domainname", obj: &v1alpha1.Route{}},
		"app-2":               {name: "e2e-app-2", obj: &v1alpha1.App{}},
		"app-service-binding": {name: "app-service-binding", obj: &v1alpha1.ServiceCredentialBinding{}},
	}

	var feat = features.New("CO-159 cloudfoundry e2e test app").Setup(
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

			return ctx
		},
	)

	// creation assess steps in dependency order, e.g., `org` before `space` as `space` depends on org`.
	var steps = [...]string{"space", "domain", "route", "app", "route-2", "app-2"}
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
				//klog.InfoS("resourced details", "cr", ft.obj)
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

	// Verify that route observations are populated in App status.
	feat.Assess("app:e2e-app routes observed in status",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			app := &v1alpha1.App{}
			cr := cfg.Client().Resources()
			if err := cr.Get(ctx, "e2e-app", cfg.Namespace(), app); err != nil {
				t.Fatalf("error getting app e2e-app: %s", err.Error())
			}
			if len(app.Status.AtProvider.Routes) == 0 {
				t.Fatalf("expected at least one route in app status, got 0")
			}
			r, ok := findRouteByHost(app.Status.AtProvider.Routes, "app-route-host-domainref")
			if !ok {
				t.Fatalf("expected route with host %q in app status, not found among %v", "app-route-host-domainref", hosts(app.Status.AtProvider.Routes))
			}
			if r.URL == "" {
				t.Errorf("expected route URL to be non-empty, got empty")
			}
			t.Logf("app e2e-app route observed: URL=%s Host=%s Protocol=%s", r.URL, r.Host, r.Protocol)
			return ctx
		},
	).Assess("app:e2e-app-2 routes observed in status",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			app := &v1alpha1.App{}
			cr := cfg.Client().Resources()
			if err := cr.Get(ctx, "e2e-app-2", cfg.Namespace(), app); err != nil {
				t.Fatalf("error getting app e2e-app-2: %s", err.Error())
			}
			if len(app.Status.AtProvider.Routes) == 0 {
				t.Fatalf("expected at least one route in app status, got 0")
			}
			r, ok := findRouteByHost(app.Status.AtProvider.Routes, "app-route-host-domainname")
			if !ok {
				t.Fatalf("expected route with host %q in app status, not found among %v", "app-route-host-domainname", hosts(app.Status.AtProvider.Routes))
			}
			if r.URL == "" {
				t.Errorf("expected route URL to be non-empty, got empty")
			}
			t.Logf("app e2e-app-2 route observed: URL=%s Host=%s Protocol=%s", r.URL, r.Host, r.Protocol)
			return ctx
		},
	)

	for _, name := range steps {
		ft, ok := feats[name]
		if !ok {
			continue
		}

		if ft.updated != nil {
			feat.Assess(name+":"+ft.name+" updated",
				func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
					ft.obj.SetName(ft.name)
					ft.obj.SetNamespace(cfg.Namespace())
					if err := wait.For(ResourceReady(cfg, ft.obj), wait.WithTimeout(10*time.Minute)); err != nil {
						t.Errorf("error waiting for resource %s to be ready: %s", ft.obj.GetName(), err.Error())
					}
					cr := cfg.Client().Resources()
					if err := cr.Get(ctx, ft.name, cfg.Namespace(), ft.obj); err != nil {
						t.Errorf("error observing resource %s: %s", ft.obj.GetName(), err.Error())
					}
					if ok, err := ft.updated(ft.obj); !ok {
						t.Errorf("resource %s not updated correctly: %s", ft.obj.GetName(), err.Error())
					}
					return ctx
				})
		}
	}

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

// findRouteByHost returns the first route matching the given host.
func findRouteByHost(routes []v1alpha1.AppRouteObservation, host string) (v1alpha1.AppRouteObservation, bool) {
	if i := slices.IndexFunc(routes, func(r v1alpha1.AppRouteObservation) bool {
		return r.Host == host
	}); i >= 0 {
		return routes[i], true
	}
	return v1alpha1.AppRouteObservation{}, false
}

// hosts collects all host values from routes for diagnostic output.
func hosts(routes []v1alpha1.AppRouteObservation) []string {
	return slices.Collect(func(yield func(string) bool) {
		for _, r := range routes {
			yield(r.Host)
		}
	})
}
