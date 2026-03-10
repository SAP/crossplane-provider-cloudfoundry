package space

import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/org"

	"github.com/SAP/xp-clifford/cli/export"
	"github.com/SAP/xp-clifford/erratt"
	"github.com/SAP/xp-clifford/yaml"
	"github.com/cloudfoundry/go-cfclient/v3/client"
	v1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	v2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func convertSpaceResource(ctx context.Context, cfClient *client.Client, space *res, evHandler export.EventHandler, resolveReferences bool) *yaml.ResourceWithComment {
	slog.Debug("converting space", "name", space.GetName())
	managedSpace := &v1alpha1.Space{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.Space_Kind,
			APIVersion: v1alpha1.CRDGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: space.GetName(),
			Annotations: map[string]string{
				"crossplane.io/external-name": space.GetGUID(),
			},
		},
		Spec: v1alpha1.SpaceSpec{
			ManagedResourceSpec: v2.ManagedResourceSpec{
				ManagementPolicies: []v1.ManagementAction{
					v1.ManagementActionObserve,
				},
			},
			ForProvider: v1alpha1.SpaceParameters{
				// AllowSSH:         false,
				Annotations:      space.Metadata.Annotations,
				IsolationSegment: new(string),
				Labels:           space.Metadata.Labels,
				Name:             space.Name,
				OrgReference: v1alpha1.OrgReference{
					Org: &space.Relationships.Organization.Data.GUID,
				},
			},
		},
	}
	spaceWithComment := yaml.NewResourceWithComment(managedSpace)
	spaceWithComment.CloneComment(space.ResourceWithComment)
	if resolveReferences {
		if err := org.ResolveReference(ctx, cfClient, &managedSpace.Spec.ForProvider.OrgReference); err != nil {
			evHandler.Warn(erratt.Errorf("cannot resolve org reference: %w", err).With("space-name", managedSpace.GetName()))
		}
	}
	return spaceWithComment
}
