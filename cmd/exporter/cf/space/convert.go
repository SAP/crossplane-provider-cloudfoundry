package space

import (
	"context"
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/cache"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/org"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/cli/export"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/erratt"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/yaml"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type spaceWithComment struct {
	*cache.ResourceWithComment
	*v1alpha1.Space
}

var _ yaml.CommentedYAML = &spaceWithComment{}

func convertSpaceResource(ctx context.Context, cfClient *client.Client, space *res, evHandler export.EventHandler, resolveReferences bool) *spaceWithComment {
	slog.Debug("converting space", "name", space.GetName())
	sp := &spaceWithComment{
		ResourceWithComment: &cache.ResourceWithComment{},
	}
	sp.CloneComment(space.ResourceWithComment)
	orgReference := v1alpha1.OrgReference{
		Org: &space.Relationships.Organization.Data.GUID,
	}
	name := space.GetName()
	if resolveReferences {
		if err := org.ResolveReference(ctx, cfClient, &orgReference); err != nil {
			evHandler.Warn(erratt.Errorf("cannot resolve org reference: %w", err).With("space-name", name))
		}
	}
	sp.Space = &v1alpha1.Space{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.Space_Kind,
			APIVersion: v1alpha1.CRDGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				"crossplane.io/external-name": space.GUID,
			},
		},
		Spec: v1alpha1.SpaceSpec{
			ResourceSpec: v1.ResourceSpec{
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
				OrgReference:     orgReference,
			},
		},
	}
	return sp
}
