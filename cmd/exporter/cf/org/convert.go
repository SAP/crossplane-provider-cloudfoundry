package org

import (
	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/cache"
	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/yaml"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type orgWithComment struct {
	*cache.ResourceWithComment
	*v1alpha1.Organization
}

var _ yaml.CommentedYAML = &orgWithComment{}

func convertOrgResource(org *res) *orgWithComment {
	o := &orgWithComment{
		ResourceWithComment: &cache.ResourceWithComment{},
	}
	o.CloneComment(org.ResourceWithComment)
	o.Organization = &v1alpha1.Organization{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.Org_Kind,
			APIVersion: v1alpha1.CRDGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: org.GetName(),
			Annotations: map[string]string{
				"crossplane.io/external-name": org.GUID,
			},
		},
		Spec: v1alpha1.OrgSpec{
			ResourceSpec: v1.ResourceSpec{
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
	}
	return o
}
