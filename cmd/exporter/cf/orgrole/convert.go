package orgrole

import (
	"context"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/org"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/userrole"

	"github.com/SAP/xp-clifford/cli/export"
	"github.com/SAP/xp-clifford/erratt"
	"github.com/SAP/xp-clifford/yaml"
	"github.com/cloudfoundry/go-cfclient/v3/client"
	v1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	v2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func convertOrgRoleResource(ctx context.Context, cfClient *client.Client, orgRole *userrole.Role, evHandler export.EventHandler, resolveReferences bool) *yaml.ResourceWithComment {
	managedOrgRole := &v1alpha1.OrgRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.OrgRole_Kind,
			APIVersion: v1alpha1.CRDGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: orgRole.GetName(),
			Annotations: map[string]string{
				"crossplane.io/external-name": orgRole.GetGUID(),
			},
		},
		Spec: v1alpha1.OrgRoleSpec{
			ManagedResourceSpec: v2.ManagedResourceSpec{
				ManagementPolicies: []v1.ManagementAction{
					v1.ManagementActionObserve,
				},
			},
			ForProvider: v1alpha1.OrgRoleParameters{
				Type:     orgRole.Type,
				Origin:   orgRole.Origin,
				Username: ptr.Deref(orgRole.Username, ""),
			},
		},
	}
	orgRoleWithComment := yaml.NewResourceWithComment(managedOrgRole)
	orgRoleWithComment.CloneComment(orgRole.ResourceWithComment)

	managedOrgRole.Spec.ForProvider.OrgReference = v1alpha1.OrgReference{
		Org: &orgRole.Relationships.Org.Data.GUID,
	}

	if resolveReferences {
		if err := org.ResolveReference(ctx, cfClient, &managedOrgRole.Spec.ForProvider.OrgReference); err != nil {
			evHandler.Warn(erratt.Errorf("cannot resolve org reference: %w", err).With("orgRole-name", orgRole.GetName(), "org-guid", orgRole.Relationships.Org.Data.GUID))
		}
	}
	return orgRoleWithComment
}
