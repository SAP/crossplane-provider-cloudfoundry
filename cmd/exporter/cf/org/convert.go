package org

import (
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/metadata"

	"github.com/SAP/xp-clifford/yaml"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func convertOrgResource(org *res) *yaml.ResourceWithComment {
	slog.Debug("converting org", "name", org.GetName())
	o := yaml.NewResourceWithComment(
		&v1alpha1.Organization{
			TypeMeta: metav1.TypeMeta{
				Kind:       v1alpha1.Org_Kind,
				APIVersion: v1alpha1.CRDGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: org.GetName(),
				Annotations: map[string]string{
					"crossplane.io/external-name": org.GetGUID(),
				},
			},
			Spec: v1alpha1.OrgSpec{
				ClusterManagedResourceSpec: xpv2.ClusterManagedResourceSpec{
					ManagementPolicies: []xpv2.ManagementAction{
						xpv2.ManagementActionObserve,
					},
				},
				ForProvider: v1alpha1.OrgParameters{
					ResourceMetadata: v1alpha1.ResourceMetadata{
						Annotations: org.Metadata.Annotations,
						Labels:      metadata.StripDefaultLabels(org.Metadata.Labels),
					},
					Name:      org.Name,
					Suspended: &org.Suspended,
				},
			},
		})
	o.CloneComment(org)
	return o
}
