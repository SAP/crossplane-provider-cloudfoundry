//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"

	v1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

func TestCloudFoundryServices(t *testing.T) {
	var dir = "./crs/service"
	var namespace = "service-test"

	targetSpaceObj := &v1alpha1.Space{}

	var feats = map[string]struct {
		// name of the managed resource
		name string
		// managed resource
		obj k8s.Object
		// updated checks if resource is updated, normally by observing a new value on managed field.
		updated func(k8s.Object) (bool, error)
	}{
		"space":        {name: "service-space", obj: &v1alpha1.Space{}},
		"target_space": {name: "service-space-shared", obj: targetSpaceObj},
		"service_instance": {
			name: "e2e-service-instance",
			obj:  &v1alpha1.ServiceInstance{},
			updated: func(obj k8s.Object) (bool, error) {
				si := obj.(*v1alpha1.ServiceInstance)
				if len(si.Spec.ForProvider.SharedSpaces) == 0 {
					return false, fmt.Errorf("expected sharedSpaces to be non-empty")
				}
				resolvedGUID := si.Spec.ForProvider.SharedSpaces[0].Space
				if resolvedGUID == nil || *resolvedGUID == "" {
					return false, fmt.Errorf("sharedSpaces[0].space GUID was not resolved (nil or empty)")
				}
				targetGUID := targetSpaceObj.Status.AtProvider.ID
				if targetGUID == "" {
					return false, fmt.Errorf("target space %q GUID not yet available in status", targetSpaceObj.GetName())
				}
				if *resolvedGUID != targetGUID {
					return false, fmt.Errorf("sharedSpaces[0].space GUID %q does not match target space %q GUID %q",
						*resolvedGUID, targetSpaceObj.GetName(), targetGUID)
				}
				return true, nil
			},
		},
		"ups":                {name: "e2e-ups", obj: &v1alpha1.ServiceInstance{}},
		"ups_no_credentials": {name: "e2e-ups-no-credentials", obj: &v1alpha1.ServiceInstance{}},
		"scb_key":            {name: "e2e-scb-key", obj: &v1alpha1.ServiceCredentialBinding{}},
	}

	var feat = features.New("CO-159 cloudfoundry e2e test services").Setup(
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

	var steps = [...]string{"space", "target_space", "service_instance", "ups", "ups_no_credentials", "scb_key"}
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
