package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	resourcesv1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	v1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1alpha1"
	v1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
)

func getScheme() (*runtime.Scheme, error) {
	scheme, err := v1alpha1.SchemeBuilder.
		RegisterAll(resourcesv1alpha1.SchemeBuilder).
		RegisterAll(v1beta1.SchemeBuilder).
		Build()
	if err != nil {
		return nil, err
	}
	err = corev1.SchemeBuilder.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	return scheme, nil
}
