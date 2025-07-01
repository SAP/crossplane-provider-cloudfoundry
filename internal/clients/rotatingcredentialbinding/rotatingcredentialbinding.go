package rotatingcredentialbinding

import (
	"context"
	"math/rand"
	"strings"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/pkg/errors"
	apicorev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ErrGetSecretTimeout = "timed out waiting for service credential binding secret to be available"
	ErrDeleteTimeout    = "timed out waiting for service credential binding to be deleted"
)

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

func DeleteSCB(ctx context.Context, kube k8s.Client, name, namespace string) error {
	var scb v1alpha1.ServiceCredentialBinding

	if err := kube.Get(ctx, k8s.ObjectKey{Namespace: namespace, Name: name}, &scb); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil // already deleted
		}
		return errors.Wrap(err, "cannot get service credential binding")
	}

	delErr := kube.Delete(ctx, &scb)
	if delErr != nil {
		if !strings.Contains(delErr.Error(), "There is an operation in progress for the service binding.") {
			return errors.Wrap(delErr, "cannot delete service credential binding")
		}
	}

	waitTimeout := 60 * time.Second
	waitInterval := 2 * time.Second
	ctxTimeout, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()
	for {
		err := kube.Get(ctxTimeout, k8s.ObjectKey{Namespace: namespace, Name: name}, &scb)
		if k8serrors.IsNotFound(err) {
			break // deleted
		}
		if err != nil {
			return errors.Wrap(err, "error waiting for service credential binding deletion")
		}
		select {
		case <-ctxTimeout.Done():
			if delErr != nil {
				return errors.Wrap(delErr, ErrDeleteTimeout)
			}
			return errors.New(ErrDeleteTimeout)
		case <-time.After(waitInterval):
			continue
		}
	}

	return nil
}

func GetSecret(ctx context.Context, kube k8s.Client, cr *v1alpha1.RotatingCredentialBinding) (*apicorev1.Secret, error) {
	if cr.Status.ActiveServiceCredentialBinding == nil {
		return nil, errors.New("active service credential binding is nil")
	}

	sourceName := cr.Status.ActiveServiceCredentialBinding.Name
	sourceNamespace := cr.Status.ActiveServiceCredentialBinding.Namespace

	var sourceSecret apicorev1.Secret
	waitTimeout := 60 * time.Second
	waitInterval := 2 * time.Second
	ctxTimeout, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()
	for {
		err := kube.Get(ctxTimeout, k8s.ObjectKey{Namespace: sourceNamespace, Name: sourceName}, &sourceSecret)
		if err == nil {
			break
		}
		if k8serrors.IsNotFound(err) {
			select {
			case <-ctxTimeout.Done():
				return nil, errors.New(ErrGetSecretTimeout)
			case <-time.After(waitInterval):
				continue
			}
		}
		return nil, errors.Wrap(err, "cannot get source secret for current binding")
	}

	return &sourceSecret, nil
}

const letterBytes = "abcdefghijklmnopqrstuvwxyz1234567890"
const (
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
