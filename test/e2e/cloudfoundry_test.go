//go:build e2e

package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	meta "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis"
	v1alpha1resources "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	v1alpha2resources "github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/apis/resources/v1alpha2"
	v1 "k8s.io/api/core/v1"
	wait2 "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	resources "sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestCloudfoundry(t *testing.T) {
	var dir = "./crs"
	var feats = map[string]struct {
		// name of the managed resource
		name string
		// managed resource
		obj k8s.Object
		// updated checks if resource is updated, normally by observing a new value on managed field.
		updated func(k8s.Object) (bool, error)
	}{
		"org":                        {name: "my-org", obj: &v1alpha2resources.Org{}},
		"org_managers":               {name: "my-org-managers", obj: &v1alpha1resources.OrgMembers{}},
		"org_role":                   {name: "my-org-role", obj: &v1alpha2resources.OrgRole{}},
		"space":                      {name: "my-space", obj: &v1alpha2resources.Space{}},
		"space_developers":           {name: "my-space-developers", obj: &v1alpha1resources.SpaceMembers{}},
		"space_role":                 {name: "my-space-role", obj: &v1alpha2resources.SpaceRole{}},
		"service_instance":           {name: "my-service-instance", obj: &v1alpha2resources.ServiceInstance{}},
		"ups":                        {name: "my-ups", obj: &v1alpha2resources.ServiceInstance{}},
		"service_credential_binding": {name: "my-service-credential-binding", obj: &v1alpha2resources.ServiceCredentialBinding{}},
		"service_key":                {name: "my-service-key", obj: &v1alpha1resources.ServiceKey{}},
		"route":                      {name: "my-route", obj: &v1alpha1resources.Route{}},
		"app":                        {name: "my-app", obj: &v1alpha2resources.App{}},
	}

	var feat = features.New("CO-159 cloudfoundry e2e test").Setup(
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			org, err := orgID(ctx, testOrgName)
			if err != nil {
				t.Fatalf("test org %s not accessible", testOrgName)
			}
			_ = deleteSpace(ctx, org, feats["space"].name)
			_ = deleteDomain(ctx, org, "dev.orchestrator.io")

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
	var steps = [...]string{"org", "org_managers", "org_role", "space", "space_role", "space_developers", "service_instance", "service_credential_binding", "ups", "app"}
	for _, name := range steps {
		ft, ok := feats[name]
		if !ok {
			continue
		}
		feat.Assess(name+":"+ft.name+" created",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				ft.obj.SetName(ft.name)
				ft.obj.SetNamespace(cfg.Namespace())
				if err := wait.For(ResourceReady(cfg, ft.obj), wait.WithTimeout(10*time.Minute)); err != nil {
					t.Errorf("error waiting for resource %s to be ready: %s", ft.obj.GetName(), err.Error())
				}
				return ctx
			},
		).Assess(name+":"+ft.name+" observed",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				//TODO: add assessment to check if resources are created on BTP
				cr := cfg.Client().Resources()
				if err := cr.Get(ctx, ft.name, cfg.Namespace(), ft.obj); err != nil {
					t.Errorf("error observing resource %s: %s", ft.obj.GetName(), err.Error())
				}
				//klog.InfoS("resourced details", "cr", ft.obj)
				return ctx
			},
		)
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
		condition := mg.GetCondition(xpv1.TypeReady)
		result := condition.Status == v1.ConditionTrue
		klog.V(4).Infof(
			"Waiting %s to become ready. Result = %v",
			mg.GetName(),
			result,
		)
		return result
	})
}

func ResourceDeleted(cfg *envconf.Config, object k8s.Object) wait2.ConditionWithContextFunc {
	var cr = cfg.Client().Resources()
	return conditions.New(cr).ResourceDeleted(object)
}

func orgID(ctx context.Context, org string) (string, error) {
	cfClient, err := getCfClient()
	if err != nil {
		klog.V(4).InfoS("cannot get connect to cloudfoundry")
		return "", err
	}

	s, err := cfClient.Organizations.Single(ctx,
		&client.OrganizationListOptions{
			Names: client.Filter{Values: []string{org}},
		})

	if err != nil {
		return "", err
	}

	return s.GUID, nil
}

func deleteSpace(ctx context.Context, org string, space string) error {
	cfClient, err := getCfClient()
	if err != nil {
		klog.V(4).InfoS("cannot get connect to cloudfoundry")
		return err
	}
	s, err := cfClient.Spaces.Single(ctx,
		&client.SpaceListOptions{
			OrganizationGUIDs: client.Filter{Values: []string{org}},
			Names:             client.Filter{Values: []string{space}},
		})
	if err == nil {
		klog.V(4).InfoS("found test space! cleaning up")
		_, err = cfClient.Spaces.Delete(ctx, s.GUID)
		return err
	}

	return nil

}

func deleteDomain(ctx context.Context, org string, domain string) error {
	cfClient, err := getCfClient()
	if err != nil {
		klog.V(4).InfoS("cannot get connect to cloudfoundry")
		return err
	}
	s, err := cfClient.Domains.Single(ctx,
		&client.DomainListOptions{
			OrganizationGUIDs: client.Filter{Values: []string{org}},
			Names:             client.Filter{Values: []string{domain}},
		})
	if err == nil {
		klog.V(4).InfoS("found test domain! cleaning up")
		_, err = cfClient.Domains.Delete(ctx, s.GUID)
		return err
	}
	return nil
}
