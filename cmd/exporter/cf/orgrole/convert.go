package orgrole

import (
	"context"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/cache"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/org"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/userrole"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/erratt"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/yaml"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

type orgRoleWithComment struct {
	*v1alpha1.OrgRole
	*cache.ResourceWithComment
}

var _ yaml.CommentedYAML = &orgRoleWithComment{}

func convertOrgRoleResource(ctx context.Context, cfClient *client.Client, orgRole *userrole.Role, evHandler export.EventHandler, resolveReferences bool) *orgRoleWithComment {
	oRole := &orgRoleWithComment{
		ResourceWithComment: &cache.ResourceWithComment{},
	}

	oRole.CloneComment(orgRole.ResourceWithComment)

	orgReference := v1alpha1.OrgReference{
		Org: &orgRole.Relationships.Org.Data.GUID,
	}

	if resolveReferences {
		if err := org.ResolveReference(ctx, cfClient, &orgReference); err != nil {
			evHandler.Warn(erratt.Errorf("cannot resolve org reference: %w", err).With("orgRole-name", orgRole.GetName()))
		}
	}
	oRole.OrgRole = &v1alpha1.OrgRole{
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
			ResourceSpec: v1.ResourceSpec{
				ManagementPolicies: []v1.ManagementAction{
					v1.ManagementActionObserve,
				},
			},
			ForProvider: v1alpha1.OrgRoleParameters{
				OrgReference: orgReference,
				Type:         orgRole.Type,
				Origin:       orgRole.Origin,
				Username:     ptr.Deref(orgRole.Username, ""),
			},
		},
	}
	return oRole
}
