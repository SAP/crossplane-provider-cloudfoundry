//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"

	v1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

func TestCloudFoundryOrgSpace(t *testing.T) {
	var dir = "./crs/orgspace"
	var namespace = "orgspace-test"
	var feats = map[string]struct {
		// name of the managed resource
		name string
		// managed resource
		obj k8s.Object
		// updated checks if resource is updated, normally by observing a new value on managed field.
		updated func(k8s.Object) (bool, error)
	}{
		"org":           {name: "e2e-org", obj: &v1alpha1.Organization{}},
		"org_role":      {name: "e2e-org-role-org-ref", obj: &v1alpha1.OrgRole{}},
		"org_role_2":    {name: "e2e-org-role-org-name", obj: &v1alpha1.OrgRole{}},
		"org_members":   {name: "e2e-org-members", obj: &v1alpha1.OrgMembers{}},
		"space":         {name: "e2e-space-org-ref", obj: &v1alpha1.Space{}},
		"space_2":       {name: "e2e-space-org-name", obj: &v1alpha1.Space{}},
		"space_role":    {name: "e2e-space-role", obj: &v1alpha1.SpaceRole{}},
		"space_members": {name: "e2e-space-members", obj: &v1alpha1.SpaceMembers{}},
		"space_quota":   {name: "e2e-space-quota", obj: &v1alpha1.SpaceQuota{}},
	}

	var feat = features.New("CO-159 cloudfoundry e2e test org and space").Setup(
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
	var steps = [...]string{"org", "org_role", "org_role_2", "org_members", "space", "space_2", "space_role", "space_members", "space_quota"}
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
