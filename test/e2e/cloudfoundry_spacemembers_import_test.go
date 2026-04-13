//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	v1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

const spaceMembersExternalNameAnnotation = "crossplane.io/external-name"

func TestSpaceMembersImport(t *testing.T) {
	const (
		dir                = "./crs/spacemembers"
		namespace          = "spacemembers-import-test"
		orgName            = "spacemembers-import-org"
		spaceName          = "spacemembers-import-space"
		spaceMembersName   = "e2e-space-members-import"
		spaceMembersRole   = "Developers"
		resourceWaitTimout = 10 * time.Minute
	)

	feat := features.New("cloudfoundry e2e import test spacemembers").Setup(
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
			_ = UnapplyResources(ctx, cfg, dir)
			return ctx
		},
	)

	feat.Assess("organization:"+orgName+" observed",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			org := &v1alpha1.Organization{}
			if err := cfg.Client().Resources().Get(ctx, orgName, cfg.Namespace(), org); err != nil {
				t.Errorf("error observing organization %s: %s", orgName, err.Error())
			}
			return ctx
		}).Assess("organization:"+orgName+" ready",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			org := &v1alpha1.Organization{}
			org.SetName(orgName)
			org.SetNamespace(cfg.Namespace())
			if err := wait.For(ResourceReady(cfg, org), wait.WithTimeout(resourceWaitTimout)); err != nil {
				t.Errorf("error waiting for organization %s to be ready: %s", orgName, err.Error())
			}
			return ctx
		})

	feat.Assess("space:"+spaceName+" observed",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			space := &v1alpha1.Space{}
			if err := cfg.Client().Resources().Get(ctx, spaceName, cfg.Namespace(), space); err != nil {
				t.Errorf("error observing space %s: %s", spaceName, err.Error())
			}
			return ctx
		}).Assess("space:"+spaceName+" ready",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			space := &v1alpha1.Space{}
			space.SetName(spaceName)
			space.SetNamespace(cfg.Namespace())
			if err := wait.For(ResourceReady(cfg, space), wait.WithTimeout(resourceWaitTimout)); err != nil {
				t.Errorf("error waiting for space %s to be ready: %s", spaceName, err.Error())
			}
			return ctx
		})

	feat.Assess("spacemembers:"+spaceMembersName+" observed",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			spaceMembers := &v1alpha1.SpaceMembers{}
			if err := cfg.Client().Resources().Get(ctx, spaceMembersName, cfg.Namespace(), spaceMembers); err != nil {
				t.Errorf("error observing space members resource %s: %s", spaceMembersName, err.Error())
			}
			return ctx
		}).Assess("spacemembers:"+spaceMembersName+" set compound external-name",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			cr := cfg.Client().Resources()

			space := &v1alpha1.Space{}
			if err := cr.Get(ctx, spaceName, cfg.Namespace(), space); err != nil {
				t.Errorf("error getting imported space %s: %s", spaceName, err.Error())
				return ctx
			}
			if space.Status.AtProvider.ID == "" {
				t.Errorf("imported space %s does not expose a GUID in status", spaceName)
				return ctx
			}

			spaceMembers := &v1alpha1.SpaceMembers{}
			if err := cr.Get(ctx, spaceMembersName, cfg.Namespace(), spaceMembers); err != nil {
				t.Errorf("error getting space members resource %s: %s", spaceMembersName, err.Error())
				return ctx
			}

			externalName := fmt.Sprintf("%s/%s", space.Status.AtProvider.ID, spaceMembersRole)
			annotations := spaceMembers.GetAnnotations()
			if annotations == nil {
				annotations = map[string]string{}
			}
			annotations[spaceMembersExternalNameAnnotation] = externalName
			spaceMembers.SetAnnotations(annotations)

			if err := cr.Update(ctx, spaceMembers); err != nil {
				t.Errorf("error updating space members resource %s with compound external-name: %s", spaceMembersName, err.Error())
			}
			return ctx
		}).Assess("spacemembers:"+spaceMembersName+" ready",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			spaceMembers := &v1alpha1.SpaceMembers{}
			spaceMembers.SetName(spaceMembersName)
			spaceMembers.SetNamespace(cfg.Namespace())
			if err := wait.For(ResourceReady(cfg, spaceMembers), wait.WithTimeout(resourceWaitTimout)); err != nil {
				t.Errorf("error waiting for space members resource %s to be ready: %s", spaceMembersName, err.Error())
			}
			return ctx
		}).Assess("spacemembers:"+spaceMembersName+" imported",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			cr := cfg.Client().Resources()

			space := &v1alpha1.Space{}
			if err := cr.Get(ctx, spaceName, cfg.Namespace(), space); err != nil {
				t.Errorf("error getting imported space %s: %s", spaceName, err.Error())
				return ctx
			}

			spaceMembers := &v1alpha1.SpaceMembers{}
			if err := cr.Get(ctx, spaceMembersName, cfg.Namespace(), spaceMembers); err != nil {
				t.Errorf("error getting imported space members resource %s: %s", spaceMembersName, err.Error())
				return ctx
			}

			expectedExternalName := fmt.Sprintf("%s/%s", space.Status.AtProvider.ID, spaceMembersRole)
			if got := spaceMembers.GetAnnotations()[spaceMembersExternalNameAnnotation]; got != expectedExternalName {
				t.Errorf("unexpected compound external-name: got %q, want %q", got, expectedExternalName)
			}
			if len(spaceMembers.Status.AtProvider.AssignedRoles) == 0 {
				t.Errorf("expected imported SpaceMembers resource %s to expose assigned roles", spaceMembersName)
			}
			if spaceMembers.Spec.ForProvider.RoleType != spaceMembersRole {
				t.Errorf("unexpected roleType after observation: got %q, want %q", spaceMembers.Spec.ForProvider.RoleType, spaceMembersRole)
			}

			return ctx
		})

	testenv.Test(t, feat.Feature())
}
