//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

func TestCloudfoundry_App(t *testing.T) {
	var dir = crsDir("app")
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
				return ctx
			}).Assess(name+":"+ft.name+" ready",
			func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
				ft.obj.SetName(ft.name)
				ft.obj.SetNamespace(cfg.Namespace())
				t.Logf("wait for resource %s to be ready", ft.obj.GetName())
				if err := wait.For(ResourceReady(cfg, ft.obj), wait.WithTimeout(10*time.Minute)); err != nil {
					t.Errorf("error waiting for resource %s to be ready: %s", ft.obj.GetName(), err.Error())
				}
				checkAppResourceLabelsAndAnnotations(ctx, t, cfg, ft.obj, name)
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
			expectedHost := runScopedName("app-route-host-domainref")
			r, ok := findRouteByHost(app.Status.AtProvider.Routes, expectedHost)
			if !ok {
				t.Fatalf("expected route with host %q in app status, not found among %v", expectedHost, hosts(app.Status.AtProvider.Routes))
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
			expectedHost := runScopedName("app-route-host-domainname")
			r, ok := findRouteByHost(app.Status.AtProvider.Routes, expectedHost)
			if !ok {
				t.Fatalf("expected route with host %q in app status, not found among %v", expectedHost, hosts(app.Status.AtProvider.Routes))
			}
			if r.URL == "" {
				t.Errorf("expected route URL to be non-empty, got empty")
			}
			t.Logf("app e2e-app-2 route observed: URL=%s Host=%s Protocol=%s", r.URL, r.Host, r.Protocol)
			return ctx
		},
	)

	feat.Assess("app:e2e-app rolling restart without downtime", assessRollingRestart)

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

// restartAnnotation is a literal so the e2e package does not import internal/.
const restartAnnotation = "app.cloudfoundry.crossplane.io/restart"

// assessRollingRestart restarts e2e-app and checks its route keeps answering
// and the restart is not repeated. Failures use Errorf so delete steps still run.
func assessRollingRestart(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
	res := cfg.Client().Resources()
	app := &v1alpha1.App{}
	if err := res.Get(ctx, "e2e-app", cfg.Namespace(), app); err != nil {
		t.Errorf("error getting app e2e-app: %s", err.Error())
		return ctx
	}
	route, ok := findRouteByHost(app.Status.AtProvider.Routes, runScopedName("app-route-host-domainref"))
	if !ok {
		t.Errorf("route of e2e-app not found among %v", hosts(app.Status.AtProvider.Routes))
		return ctx
	}
	url := "https://" + route.URL
	httpClient := &http.Client{Timeout: 5 * time.Second}
	reachable := func() bool {
		resp, err := httpClient.Get(url)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode >= 200 && resp.StatusCode < 300
	}

	// Baseline: the app must answer before the restart, or the probe proves nothing.
	if err := wait.For(func(context.Context) (bool, error) { return reachable(), nil },
		wait.WithTimeout(3*time.Minute), wait.WithInterval(5*time.Second)); err != nil {
		t.Errorf("app e2e-app not reachable at %s before restart: %s", url, err.Error())
		return ctx
	}

	// Probe once per second; a probe fails only if an immediate retry fails too.
	var (
		mu       sync.Mutex
		failures []time.Time
		probes   sync.WaitGroup
		stopOnce sync.Once
	)
	stop := make(chan struct{})
	probes.Add(1)
	go func() {
		defer probes.Done()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if !reachable() && !reachable() {
					mu.Lock()
					failures = append(failures, time.Now().UTC())
					mu.Unlock()
				}
			}
		}
	}()
	stopProbes := func() {
		stopOnce.Do(func() {
			close(stop)
			probes.Wait()
		})
	}
	defer stopProbes()

	value := time.Now().UTC().Format(time.RFC3339Nano)
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := res.Get(ctx, "e2e-app", cfg.Namespace(), app); err != nil {
			return err
		}
		annotations := app.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[restartAnnotation] = value
		app.SetAnnotations(annotations)
		return res.Update(ctx, app)
	}); err != nil {
		t.Errorf("error setting restart annotation on e2e-app: %s", err.Error())
		return ctx
	}
	t.Logf("requested rolling restart of e2e-app with %s=%s", restartAnnotation, value)

	if err := wait.For(conditions.New(res).ResourceMatch(app, func(o k8s.Object) bool {
		d := o.(*v1alpha1.App).Status.AtProvider.Deployment
		return d != nil && d.Restart == value && d.State == "FINALIZED"
	}), wait.WithTimeout(10*time.Minute)); err != nil {
		t.Errorf("rolling restart of e2e-app did not finish: %s (deployment: %+v)", err.Error(), app.Status.AtProvider.Deployment)
		return ctx
	}
	// A copy: the Get below decodes into app and may reuse its pointers.
	deployment := *app.Status.AtProvider.Deployment
	if deployment.Reason != "DEPLOYED" {
		t.Errorf("rolling deployment %s of e2e-app ended with reason %s, want DEPLOYED", deployment.GUID, deployment.Reason)
		return ctx
	}

	// Keep probing: CF may remove the old instances just before the provider sees FINALIZED.
	t.Logf("rolling deployment %s finished; checking it is not repeated", deployment.GUID)
	select {
	case <-ctx.Done():
		t.Errorf("context done while checking the restart is not repeated: %s", ctx.Err())
		return ctx
	case <-time.After(90 * time.Second):
	}
	stopProbes()
	if len(failures) > 0 {
		t.Errorf("e2e-app at %s was unreachable during or after the rolling restart at %v", url, failures)
	}

	// Idempotency: the same annotation value must not start another deployment.
	if err := res.Get(ctx, "e2e-app", cfg.Namespace(), app); err != nil {
		t.Errorf("error getting app e2e-app: %s", err.Error())
		return ctx
	}
	if got := app.Status.AtProvider.Deployment; got == nil || got.GUID != deployment.GUID {
		t.Errorf("restart was repeated: deployment %+v, want guid %s", got, deployment.GUID)
	}
	if got := app.Status.AtProvider.LastHandledRestart; got != value {
		t.Errorf("lastHandledRestart = %q, want %q", got, value)
	}
	return ctx
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

func checkAppResourceLabelsAndAnnotations(ctx context.Context, t *testing.T, cfg *envconf.Config, obj k8s.Object, stepName string) {
	cr := cfg.Client().Resources()
	switch v := obj.(type) {
	case *v1alpha1.App:
		if err := cr.Get(ctx, v.GetName(), cfg.Namespace(), v); err != nil {
			t.Errorf("error getting App for label check: %s", err.Error())
			return
		}
		description := "E2E test app"
		if stepName == "app-2" {
			description = "E2E test app 2"
		}
		if err := AssertLabelsAndAnnotations(
			v.Status.AtProvider.Labels,
			v.Status.AtProvider.Annotations,
			map[string]string{"environment": "test", "team": "platform"},
			map[string]string{"description": description},
			v.GetName(),
			"app.cloudfoundry.crossplane.io",
			v.GetProviderConfigReference().Name,
		); err != nil {
			t.Errorf("App %s labels/annotations check failed: %s", v.GetName(), err.Error())
		}
	default:
		// Observe-only resources (Space, Domain, ServiceInstance, SCB) and other non-eligible types — skip
	}
}
