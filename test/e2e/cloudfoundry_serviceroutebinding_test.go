//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
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
				checkSRBResourceLabelsAndAnnotations(ctx, t, cfg, ft.obj)
				return ctx
			})
	}

	// Test metadata update: decode updated manifest, then update the existing CR
	feat.Assess("service_route_binding:e2e-serviceroutebinding-binding apply metadata update",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			// Decode the updated manifest to get the desired labels/annotations
			updated := &v1alpha1.ServiceRouteBinding{}
			err := decoder.DecodeEachFile(
				ctx, os.DirFS(dir+"/updated"), "serviceroutebinding-updated.yaml",
				func(ctx context.Context, obj k8s.Object) error {
					updated = obj.(*v1alpha1.ServiceRouteBinding)
					return nil
				},
				decoder.MutateNamespace(cfg.Namespace()),
			)
			if err != nil {
				t.Fatalf("error decoding updated manifest: %s", err.Error())
			}

			// Get the existing CR (preserving resourceVersion)
			existing := &v1alpha1.ServiceRouteBinding{}
			cr := cfg.Client().Resources()
			if err := cr.Get(ctx, "e2e-serviceroutebinding-binding", cfg.Namespace(), existing); err != nil {
				t.Fatalf("error getting existing SRB: %s", err.Error())
			}

			// Apply updated labels/annotations from the manifest
			existing.Spec.ForProvider.Labels = updated.Spec.ForProvider.Labels
			existing.Spec.ForProvider.Annotations = updated.Spec.ForProvider.Annotations

			if err := cr.Update(ctx, existing); err != nil {
				t.Fatalf("error updating SRB with new labels: %s", err.Error())
			}
			t.Logf("Applied updated metadata to SRB")
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
			// Poll until updated labels/annotations appear in status
			if err := wait.For(func(ctx context.Context) (bool, error) {
				cr := cfg.Client().Resources()
				if err := cr.Get(ctx, "e2e-serviceroutebinding-binding", cfg.Namespace(), binding); err != nil {
					return false, err
				}
				if err := AssertLabelsAndAnnotations(
					binding.Status.AtProvider.Labels,
					binding.Status.AtProvider.Annotations,
					map[string]string{"environment": "production", "team": "operations"},
					map[string]string{"description": "Updated metadata"},
					binding.GetName(),
					"serviceroutebinding.cloudfoundry.crossplane.io",
					binding.GetProviderConfigReference().Name,
				); err != nil {
					return false, nil // not yet reconciled
				}
				return true, nil
			}, wait.WithTimeout(5*time.Minute)); err != nil {
				t.Errorf("SRB after update labels/annotations check failed: %s", err.Error())
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

// checkSRBResourceLabelsAndAnnotations verifies default Crossplane labels
// for all eligible resources in the SRB test, and user-provided labels/annotations
// for resources that have them in their manifests.
func checkSRBResourceLabelsAndAnnotations(ctx context.Context, t *testing.T, cfg *envconf.Config, obj k8s.Object) {
	cr := cfg.Client().Resources()
	switch v := obj.(type) {
	case *v1alpha1.ServiceRouteBinding:
		if err := cr.Get(ctx, v.GetName(), cfg.Namespace(), v); err != nil {
			t.Errorf("error getting SRB for label check: %s", err.Error())
			return
		}
		if err := AssertLabelsAndAnnotations(
			v.Status.AtProvider.Labels,
			v.Status.AtProvider.Annotations,
			map[string]string{"environment": "test", "team": "platform"},
			map[string]string{"description": "Initial metadata"},
			v.GetName(),
			"serviceroutebinding.cloudfoundry.crossplane.io",
			v.GetProviderConfigReference().Name,
		); err != nil {
			t.Errorf("SRB %s labels/annotations check failed: %s", v.GetName(), err.Error())
		}
	case *v1alpha1.ServiceInstance:
		if err := cr.Get(ctx, v.GetName(), cfg.Namespace(), v); err != nil {
			t.Errorf("error getting SI for label check: %s", err.Error())
			return
		}
		if err := AssertDefaultLabels(
			v.Status.AtProvider.Labels,
			v.GetName(),
			"serviceinstance.cloudfoundry.crossplane.io",
			v.GetProviderConfigReference().Name,
		); err != nil {
			t.Errorf("SI %s default labels check failed: %s", v.GetName(), err.Error())
		}
	case *v1alpha1.Route:
		if err := cr.Get(ctx, v.GetName(), cfg.Namespace(), v); err != nil {
			t.Errorf("error getting Route for label check: %s", err.Error())
			return
		}
		if err := AssertDefaultLabels(
			v.Status.AtProvider.Labels,
			v.GetName(),
			"route.cloudfoundry.crossplane.io",
			v.GetProviderConfigReference().Name,
		); err != nil {
			t.Errorf("Route %s default labels check failed: %s", v.GetName(), err.Error())
		}
	default:
		// Observe-only resources (Space, Domain, Organization) and non-eligible types — skip
	}
}
