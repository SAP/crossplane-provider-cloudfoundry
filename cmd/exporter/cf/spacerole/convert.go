package spacerole

import (
	"context"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/space"
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

func convertSpaceRoleResource(ctx context.Context, cfClient *client.Client, spRole *userrole.Role, evHandler export.EventHandler, resolveReferences bool) *yaml.ResourceWithComment {
	sRole := yaml.NewResourceWithComment(nil)

	spaceReference := v1alpha1.SpaceReference{
		Space: &spRole.Relationships.Space.Data.GUID,
	}

	if resolveReferences {
		if err := space.ResolveReference(ctx, cfClient, &spaceReference); err != nil {
			evHandler.Warn(erratt.Errorf("cannot resolve space reference: %w", err).With("spaceRole-name", spRole.GetName(), "space-guid", spRole.Relationships.Space.Data.GUID))
		}
	}

	sRole.CloneComment(spRole.ResourceWithComment)

	sRole.SetResource(&v1alpha1.SpaceRole{
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
			ManagedResourceSpec: v2.ManagedResourceSpec{
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
	})
	return sRole
}
