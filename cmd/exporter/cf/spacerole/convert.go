package spacerole

import (
	"context"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/cache"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/space"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/userrole"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/erratt"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/yaml"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

type spaceRoleWithComment struct {
	*v1alpha1.SpaceRole
	*cache.ResourceWithComment
}

var _ yaml.CommentedYAML = &spaceRoleWithComment{}

func convertSpaceRoleResource(ctx context.Context, cfClient *client.Client, spRole *userrole.Role, evHandler export.EventHandler, resolveReferences bool) *spaceRoleWithComment {
	sRole := &spaceRoleWithComment{
		ResourceWithComment: &cache.ResourceWithComment{},
	}

	spaceReference := v1alpha1.SpaceReference{
		Space: &spRole.Relationships.Space.Data.GUID,
	}

	if resolveReferences {
		if err := space.ResolveReference(ctx, cfClient, &spaceReference); err != nil {
			evHandler.Warn(erratt.Errorf("cannot resolve space reference: %w", err).With("spaceRole-name", spRole.GetName(), "space-guid", spRole.Relationships.Space.Data.GUID))
		}
	}

	sRole.CloneComment(spRole.ResourceWithComment)

	sRole.SpaceRole = &v1alpha1.SpaceRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.SpaceRole_Kind,
			APIVersion: v1alpha1.CRDGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: spRole.GetName(),
			Annotations: map[string]string{
				"crossplane.io/external-name": spRole.GetGUID(),
			},
		},
		Spec: v1alpha1.SpaceRoleSpec{
			ResourceSpec: v1.ResourceSpec{
				ManagementPolicies: []v1.ManagementAction{
					v1.ManagementActionObserve,
				},
			},
			ForProvider: v1alpha1.SpaceRoleParameters{
				SpaceReference: spaceReference,
				Type:           spRole.Type,
				Origin:         spRole.Origin,
				Username:       ptr.Deref(spRole.Username, ""),
			},
		},
	}
	return sRole
}
