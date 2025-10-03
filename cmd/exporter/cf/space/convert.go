package space

import (
	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

func convertSpaceResource(space *resource.Space) *v1alpha1.Space {
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
				ManagementPolicies:               []v1.ManagementAction{
					v1.ManagementActionObserve,
				},
			},
			ForProvider:  v1alpha1.SpaceParameters{
				// AllowSSH:         false,
				Annotations:      space.Metadata.Annotations,
				IsolationSegment: new(string),
				Labels:           space.Metadata.Labels,
				Name:             space.Name,
				OrgReference:     v1alpha1.OrgReference{
					Org:         &space.Relationships.Organization.Data.GUID,
				},
			},
		},
	}
}
