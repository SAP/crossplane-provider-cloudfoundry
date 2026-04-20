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

const orgMembersExternalNameAnnotation = "crossplane.io/external-name"

func TestOrgMembersImport(t *testing.T) {
	const (
		dir                = "./crs/orgmembers"
		namespace          = "orgmembers-import-test"
		orgName            = "orgmembers-import-org"
		orgMembersName     = "e2e-org-members-import"
		orgMembersRole     = "Managers"
		resourceWaitTimout = 10 * time.Minute
	)

	feat := features.New("cloudfoundry e2e import test orgmembers").Setup(
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

	feat.Assess("orgmembers:"+orgMembersName+" observed",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			orgMembers := &v1alpha1.OrgMembers{}
			if err := cfg.Client().Resources().Get(ctx, orgMembersName, cfg.Namespace(), orgMembers); err != nil {
				t.Errorf("error observing org members resource %s: %s", orgMembersName, err.Error())
			}
			return ctx
		}).Assess("orgmembers:"+orgMembersName+" set compound external-name",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			cr := cfg.Client().Resources()

			org := &v1alpha1.Organization{}
			if err := cr.Get(ctx, orgName, cfg.Namespace(), org); err != nil {
				t.Errorf("error getting imported organization %s: %s", orgName, err.Error())
				return ctx
			}
			if org.Status.AtProvider.ID == nil || *org.Status.AtProvider.ID == "" {
				t.Errorf("imported organization %s does not expose a GUID in status", orgName)
				return ctx
			}

			orgMembers := &v1alpha1.OrgMembers{}
			if err := cr.Get(ctx, orgMembersName, cfg.Namespace(), orgMembers); err != nil {
				t.Errorf("error getting org members resource %s: %s", orgMembersName, err.Error())
				return ctx
			}

			externalName := fmt.Sprintf("%s/%s", *org.Status.AtProvider.ID, orgMembersRole)
			annotations := orgMembers.GetAnnotations()
			if annotations == nil {
				annotations = map[string]string{}
			}
			annotations[orgMembersExternalNameAnnotation] = externalName
			orgMembers.SetAnnotations(annotations)

			if err := cr.Update(ctx, orgMembers); err != nil {
				t.Errorf("error updating org members resource %s with compound external-name: %s", orgMembersName, err.Error())
			}
			return ctx
		}).Assess("orgmembers:"+orgMembersName+" ready",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			orgMembers := &v1alpha1.OrgMembers{}
			orgMembers.SetName(orgMembersName)
			orgMembers.SetNamespace(cfg.Namespace())
			if err := wait.For(ResourceReady(cfg, orgMembers), wait.WithTimeout(resourceWaitTimout)); err != nil {
				t.Errorf("error waiting for org members resource %s to be ready: %s", orgMembersName, err.Error())
			}
			return ctx
		}).Assess("orgmembers:"+orgMembersName+" imported",
		func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			cr := cfg.Client().Resources()

			org := &v1alpha1.Organization{}
			if err := cr.Get(ctx, orgName, cfg.Namespace(), org); err != nil {
				t.Errorf("error getting imported organization %s: %s", orgName, err.Error())
				return ctx
			}

			orgMembers := &v1alpha1.OrgMembers{}
			if err := cr.Get(ctx, orgMembersName, cfg.Namespace(), orgMembers); err != nil {
				t.Errorf("error getting imported org members resource %s: %s", orgMembersName, err.Error())
				return ctx
			}

			if org.Status.AtProvider.ID == nil || *org.Status.AtProvider.ID == "" {
				t.Errorf("imported organization %s does not expose a GUID in status", orgName)
				return ctx
			}

			expectedExternalName := fmt.Sprintf("%s/%s", *org.Status.AtProvider.ID, orgMembersRole)
			if got := orgMembers.GetAnnotations()[orgMembersExternalNameAnnotation]; got != expectedExternalName {
				t.Errorf("unexpected compound external-name: got %q, want %q", got, expectedExternalName)
			}
			if orgMembers.Spec.ForProvider.RoleType != orgMembersRole {
				t.Errorf("unexpected roleType after observation: got %q, want %q", orgMembers.Spec.ForProvider.RoleType, orgMembersRole)
			}

			return ctx
		})

	testenv.Test(t, feat.Feature())
}
