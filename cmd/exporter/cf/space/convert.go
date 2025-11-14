package space

import (
	"context"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/org"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/export"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func convertSpaceResource(ctx context.Context, cfClient *client.Client, space *resource.Space, evHandler export.EventHandler, resolveReferences bool) *v1alpha1.Space {
	orgReference := v1alpha1.OrgReference{
		Org: &space.Relationships.Organization.Data.GUID,
	}
	if resolveReferences {
		if err := org.Org.ResolveReference(ctx, cfClient, &orgReference); err != nil {
			evHandler.Warn(erratt.Errorf("cannot resolve org reference: %w", err).With("space-name", space.Name))
		}
	}
	return &v1alpha1.Space{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.Space_Kind,
			APIVersion: v1alpha1.CRDGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: space.Name,
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
}
