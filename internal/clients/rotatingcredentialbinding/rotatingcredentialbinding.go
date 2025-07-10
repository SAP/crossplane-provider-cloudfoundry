package rotatingcredentialbinding

import (
	"context"
	"math/rand"
	"strings"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/resource"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/pkg/errors"
	apicorev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

const (
	OwnerLabelKey = "rotatingcredentialbinding.cloudfoundry.crossplane.io/owner"
)

func GetAllBindings(ctx context.Context, kube k8s.Client, cr *v1alpha1.RotatingCredentialBinding) ([]v1alpha1.ServiceCredentialBinding, error) {
	labelSelector := labels.Set{
		OwnerLabelKey: string(cr.GetUID()),
	}.AsSelector()
	listOpts := []k8s.ListOption{
		k8s.MatchingLabelsSelector{Selector: labelSelector},
	}

	var retiredSCB v1alpha1.ServiceCredentialBindingList
	if err := kube.List(ctx, &retiredSCB, listOpts...); err != nil {
		return nil, errors.Wrap(err, "cannot list retired service credential bindings")
	}

	return retiredSCB.Items, nil
}

func GenerateSCB(ctx context.Context, kube k8s.Client, cr *v1alpha1.RotatingCredentialBinding, name, namespace string) (string, error) {
	name, err := randomName(ctx, kube, name, namespace)
	if err != nil {
		return "", errors.Wrap(err, "cannot generate a unique name for service credential binding")
	}
	return CreateSCB(ctx, kube, cr, name, namespace)
}

func CreateSCB(ctx context.Context, kube k8s.Client, cr *v1alpha1.RotatingCredentialBinding, name, namespace string) (string, error) {
	controller, blockOwnerDeletion := true, true
	scb := &v1alpha1.ServiceCredentialBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         v1alpha1.RotatingCredentialBindingGroupVersionKind.GroupVersion().String(),
					Kind:               v1alpha1.RotatingCredentialBindingGroupVersionKind.Kind,
					Name:               cr.GetName(),
					UID:                cr.GetUID(),
					Controller:         &controller,
					BlockOwnerDeletion: &blockOwnerDeletion,
				},
			},
			Labels: map[string]string{
				OwnerLabelKey: string(cr.GetUID()),
			},
		},
		Spec: v1alpha1.ServiceCredentialBindingSpec{
			ForProvider: v1alpha1.ServiceCredentialBindingParameters{
				Type:                    "key",
				Name:                    &name,
				ServiceInstanceRef:      cr.Spec.ForProvider.ServiceInstanceRef,
				ServiceInstanceSelector: cr.Spec.ForProvider.ServiceInstanceSelector,
				ServiceInstance:         cr.Spec.ForProvider.ServiceInstance,
				Parameters:              cr.Spec.ForProvider.Parameters,
				ParametersSecretRef:     cr.Spec.ForProvider.ParametersSecretRef,
			},
			ConnectionDetailsAsJSON: cr.Spec.ConnectionDetailsAsJSON,
			ResourceSpec: xpv1.ResourceSpec{
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Name:      name,
					Namespace: namespace,
				},
			},
		},
	}

	if err := kube.Create(ctx, scb); err != nil {
		return "", errors.Wrap(err, "cannot create service credential binding")
	}

	return name, nil
}

func DeleteSCBs(ctx context.Context, kube k8s.Client, retiredSCBs []v1alpha1.ServiceCredentialBinding, cr *v1alpha1.RotatingCredentialBinding) error {
	var delErr error
	for _, prevSCB := range retiredSCBs {
		if cr != nil && prevSCB.Name == cr.Status.ActiveServiceCredentialBinding.Name &&
			prevSCB.Namespace == cr.Status.ActiveServiceCredentialBinding.Namespace {
			// If the previous SCB is the same as the active one, we do not delete it.
			continue
		}
		if cr == nil || prevSCB.GetCreationTimestamp().Add(cr.Spec.RotationTTL.Duration).Before(time.Now()) {
			if err := kube.Delete(ctx, &prevSCB); err != nil {
				if resource.IsAsyncServiceInstanceOperationInProgressError(err) {
					delErr = errors.Wrap(err, "cannot delete old service credential binding")
					continue
				} else {
					return errors.Wrap(err, "cannot delete old service credential binding")
				}
			}
		}
	}
	if delErr != nil {
		// If deletion failed due to another operation in progress, wait before retrying.
		time.Sleep(5 * time.Second)
	}
	return delErr
}

const (
	letterBytes   = "abcdefghijklmnopqrstuvwxyz1234567890"
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

var src = rand.NewSource(time.Now().UnixNano())

func randomName(ctx context.Context, kube k8s.Client, name, namespace string) (string, error) {
	if len(name) > 0 && name[len(name)-1] == '-' {
		name = name[:len(name)-1]
	}

	for range 8 {
		res := name + "-" + randomString(5)
		if err := kube.Get(ctx, k8s.ObjectKey{Name: res, Namespace: namespace}, &apicorev1.Secret{}); err != nil {
			if k8serrors.IsNotFound(err) {
				return res, nil // name is available
			}
			return "", errors.Wrapf(err, "cannot check if name %s is available", res)
		}
	}

	return "", errors.New("cannot generate a unique name for secret, please try again")
}

func randomString(n int) string {
	sb := strings.Builder{}
	sb.Grow(n)

	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			sb.WriteByte(letterBytes[idx])
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return sb.String()
}
