package org

import (
	"log/slog"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"

	"github.com/SAP/xp-clifford/yaml"
	v1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	v2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
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
				ManagedResourceSpec: v2.ManagedResourceSpec{
					ManagementPolicies: []v1.ManagementAction{
						v1.ManagementActionObserve,
					},
				},
				ForProvider: v1alpha1.OrgParameters{
					Annotations: org.Metadata.Annotations,
					Labels:      org.Metadata.Labels,
					Name:        org.Name,
					Suspended:   &org.Suspended,
				},
			},
		})
	o.CloneComment(org)
	return o
}
