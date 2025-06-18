package rotatingcredentialbinding

import (
	"context"
	"time"

	rcb "github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/rotatingcredentialbinding"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/pkg/errors"
	apicorev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	apisv1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1alpha1"
	apisv1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/features"
)

const (
	resourceType    = "RotatingCredentialBinding"
	externalSystem  = "Cloud Foundry"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errNewClient    = "cannot create a client for " + externalSystem
	errWrongCRType  = "managed resource is not a " + resourceType
	errGet          = "cannot get " + resourceType + " in " + externalSystem
	errFind         = "cannot find " + resourceType + " in " + externalSystem
	errCreate       = "cannot create " + resourceType + " in " + externalSystem
	errUpdate       = "cannot update " + resourceType + " in " + externalSystem
	errDelete       = "cannot delete " + resourceType + " in " + externalSystem

	msgUpToDate         = "credentials are up to date"
	msgRotationDue      = "credentials are due for rotation"
	msgDeleteOld        = "credentials are up to date, but old credentials have to be deleted"
	msgSCBNotFound      = "service credential binding not found"
	msgConnOutdated     = "connection details out of date"
	msgPrevNotFound     = "previous service credential binding not found"
	msgWaitingForSecret = "waiting for source secret to be created"
)

// Setup adds a controller that reconciles RotatingCredentialBinding CR.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.RotatingCredentialBindingGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	options := []managed.ReconcilerOption{
		managed.WithInitializers(),
		managed.WithExternalConnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1beta1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...),
		managed.WithPollInterval(o.PollInterval),
	}

	if o.Features.Enabled(features.EnableBetaManagementPolicies) {
		options = append(options, managed.WithManagementPolicies())
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.RotatingCredentialBindingGroupVersionKind),
		options...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.RotatingCredentialBinding{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an external client when its Connect method
// is called.
type connector struct {
	kube  k8s.Client
	usage resource.Tracker
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	if _, ok := mg.(*v1alpha1.RotatingCredentialBinding); !ok {
		return nil, errors.New(errWrongCRType)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	return &external{
		kube: c.kube,
	}, nil
}

// An external service observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	kube k8s.Client
}

// Observe checks the observed state of the resource and updates the managed resource's status.
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.RotatingCredentialBinding)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errWrongCRType)
	}

	if cr.Status.ActiveServiceCredentialBinding == nil {
		cr.SetConditions(xpv1.Creating().WithMessage("no active secret binding found"))
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	name := cr.Status.ActiveServiceCredentialBinding.Name
	namespace := cr.Status.ActiveServiceCredentialBinding.Namespace

	var scb v1alpha1.ServiceCredentialBinding
	if err := c.kube.Get(ctx, k8s.ObjectKey{Namespace: namespace, Name: name}, &scb); err != nil {
		if k8serrors.IsNotFound(err) {
			cr.SetConditions(xpv1.Unavailable().WithMessage(msgSCBNotFound))
			return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
		}
		cr.SetConditions(xpv1.Unavailable().WithMessage("cannot get active service credential binding"))
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot get active service credential binding")
	}

	if cr.Status.ActiveServiceCredentialBinding.LastRotation.Add(cr.Spec.RotationFrequency.Duration).Before(time.Now()) {
		cr.SetConditions(xpv1.Available().WithMessage(msgRotationDue))
		return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
	}

	for _, prevSCB := range cr.Status.PreviousServiceCredentialBindings {
		if prevSCB.LastRotation.Add(cr.Spec.RotationTTL.Duration).Before(time.Now()) {
			if err := c.kube.Get(ctx, k8s.ObjectKey{Namespace: prevSCB.Namespace, Name: prevSCB.Name}, &v1alpha1.ServiceCredentialBinding{}); err != nil {
				if !k8serrors.IsNotFound(err) {
					cr.SetConditions(xpv1.Unavailable().WithMessage("cannot get backup service credential binding"))
					return managed.ExternalObservation{}, errors.Wrap(err, "cannot get backup service credential binding")
				}
				cr.SetConditions(xpv1.Available().WithMessage(msgPrevNotFound))
				return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
			} else {
				cr.SetConditions(xpv1.Available().WithMessage(msgDeleteOld))
				return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
			}
		}
	}

	if cr.Spec.WriteConnectionSecretToReference != nil && cr.Spec.WriteConnectionSecretToReference.Name != "" {
		var sourceSecret apicorev1.Secret
		if err := c.kube.Get(ctx, k8s.ObjectKey{Namespace: scb.Spec.WriteConnectionSecretToReference.Namespace, Name: scb.Spec.WriteConnectionSecretToReference.Name}, &sourceSecret); err != nil {
			if k8serrors.IsNotFound(err) {
				cr.SetConditions(xpv1.Available().WithMessage(msgConnOutdated))
				return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
			}
			cr.SetConditions(xpv1.Unavailable().WithMessage("cannot get source secret for current binding"))
			return managed.ExternalObservation{}, errors.Wrap(err, "cannot get source secret for current binding")
		}

		if len(sourceSecret.Data) == 0 {
			cr.SetConditions(xpv1.Available().WithMessage(msgWaitingForSecret))
			return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
		}

		var secret apicorev1.Secret
		if cr.Spec.WriteConnectionSecretToReference.Namespace == "" {
			cr.Spec.WriteConnectionSecretToReference.Namespace = cr.Namespace
		}
		if err := c.kube.Get(ctx, k8s.ObjectKey{Namespace: cr.Spec.WriteConnectionSecretToReference.Namespace, Name: cr.Spec.WriteConnectionSecretToReference.Name}, &secret); err != nil {
			if k8serrors.IsNotFound(err) {
				cr.SetConditions(xpv1.Available().WithMessage(msgConnOutdated))
				return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ConnectionDetails: sourceSecret.Data}, nil
			}
			cr.SetConditions(xpv1.Unavailable().WithMessage("cannot get secret for current binding"))
			return managed.ExternalObservation{}, errors.Wrap(err, "cannot get secret for current binding")
		}

		if len(secret.Data) != len(sourceSecret.Data) {
			cr.SetConditions(xpv1.Available().WithMessage(msgConnOutdated))
			return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ConnectionDetails: sourceSecret.Data}, nil
		}
		for k, v := range sourceSecret.Data {
			if val, ok := secret.Data[k]; !ok || string(val) != string(v) {
				cr.SetConditions(xpv1.Available().WithMessage(msgConnOutdated))
				return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ConnectionDetails: sourceSecret.Data}, nil
			}
		}
	}

	cr.SetConditions(xpv1.Available().WithMessage(msgUpToDate))
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil

}

// Create a RotatingCredentialBinding resource.
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.RotatingCredentialBinding)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errWrongCRType)
	}

	cr.SetConditions(xpv1.Creating())

	name := cr.Spec.ForProvider.Name
	var namespace string
	if cr.Spec.ForProvider.Namespace != nil && *cr.Spec.ForProvider.Namespace != "" {
		namespace = *cr.Spec.ForProvider.Namespace
	} else {
		namespace = cr.GetNamespace()
		if namespace == "" {
			namespace = "default"
		}
	}

	newName, err := rcb.GenerateSCB(ctx, c.kube, cr, name, namespace)
	if err != nil {
		cr.SetConditions(xpv1.Unavailable().WithMessage("cannot create service credential binding"))
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	cr.Status.ActiveServiceCredentialBinding = &v1alpha1.ServiceCredentialBindingReference{
		Name:         newName,
		Namespace:    namespace,
		LastRotation: metav1.Time{Time: time.Now()},
	}
	cr.SetConditions(xpv1.Available().WithMessage(msgUpToDate))

	if err := c.kube.Status().Update(ctx, cr); err != nil {
		cr.SetConditions(xpv1.Unavailable().WithMessage("cannot update status"))
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot update status")
	}

	if cr.Spec.WriteConnectionSecretToReference != nil && cr.Spec.WriteConnectionSecretToReference.Name != "" {
		secret, err := rcb.GetSecret(ctx, c.kube, cr)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				cr.SetConditions(xpv1.Available().WithMessage("waiting for secret to be created"))
				return managed.ExternalCreation{
					ConnectionDetails: nil,
				}, nil
			}
			cr.SetConditions(xpv1.Unavailable().WithMessage("cannot update copy secret"))
			return managed.ExternalCreation{}, errors.Wrap(err, "cannot update copy secret")
		}
		return managed.ExternalCreation{
			ConnectionDetails: secret.Data,
		}, nil
	}
	return managed.ExternalCreation{
		ConnectionDetails: nil,
	}, nil
}

// Update a RotatingCredentialBinding resource.
func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.RotatingCredentialBinding)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errWrongCRType)
	}

	name := cr.Spec.ForProvider.Name
	var namespace string
	if cr.Spec.ForProvider.Namespace != nil && *cr.Spec.ForProvider.Namespace != "" {
		namespace = *cr.Spec.ForProvider.Namespace
	} else {
		namespace = cr.GetNamespace()
		if namespace == "" {
			namespace = "default"
		}
	}

	switch msg := cr.GetCondition(xpv1.Available().Type).Message; msg {
	case msgUpToDate:
		return managed.ExternalUpdate{}, nil
	case msgRotationDue:
		newName, err := rcb.GenerateSCB(ctx, c.kube, cr, name, namespace)
		if err != nil {
			cr.SetConditions(xpv1.Unavailable().WithMessage("cannot create new service credential binding"))
			return managed.ExternalUpdate{}, errors.Wrap(err, "cannot create new service credential binding")
		}
		oldSCB := cr.Status.ActiveServiceCredentialBinding
		if oldSCB == nil {
			cr.SetConditions(xpv1.Unavailable().WithMessage("active service credential binding is nil"))
			return managed.ExternalUpdate{}, errors.New("active service credential binding is nil")
		}
		oldSCB.LastRotation = metav1.Time{Time: time.Now()}
		cr.Status.PreviousServiceCredentialBindings = append(cr.Status.PreviousServiceCredentialBindings, oldSCB)
		cr.Status.ActiveServiceCredentialBinding = &v1alpha1.ServiceCredentialBindingReference{
			Name:         newName,
			Namespace:    namespace,
			LastRotation: metav1.Time{Time: time.Now()},
		}

		if cr.Spec.WriteConnectionSecretToReference != nil && cr.Spec.WriteConnectionSecretToReference.Name != "" {
			secret, err := rcb.GetSecret(ctx, c.kube, cr)
			if err != nil {
				if k8serrors.IsNotFound(err) {
					cr.SetConditions(xpv1.Available().WithMessage("waiting for new secret to be created"))
					return managed.ExternalUpdate{
						ConnectionDetails: nil,
					}, nil
				}
				cr.SetConditions(xpv1.Unavailable().WithMessage("cannot update copy secret"))
				return managed.ExternalUpdate{}, errors.Wrap(err, "cannot update copy secret")
			}
			return managed.ExternalUpdate{
				ConnectionDetails: secret.Data,
			}, nil
		}
		return managed.ExternalUpdate{
			ConnectionDetails: nil,
		}, nil
	case msgDeleteOld:
		newPrevSCBs := make([]*v1alpha1.ServiceCredentialBindingReference, 0, len(cr.Status.PreviousServiceCredentialBindings))
		for _, prevSCB := range cr.Status.PreviousServiceCredentialBindings {
			if prevSCB.LastRotation.Add(cr.Spec.RotationTTL.Duration).Before(time.Now()) {
				if err := rcb.DeleteSCB(ctx, c.kube, prevSCB.Name, prevSCB.Namespace); err != nil && !k8serrors.IsNotFound(err) {
					cr.SetConditions(xpv1.Unavailable().WithMessage("cannot delete service credential binding"))
					return managed.ExternalUpdate{}, errors.Wrap(err, "cannot delete service credential binding")
				}
			} else {
				newPrevSCBs = append(newPrevSCBs, prevSCB)
			}
		}
		cr.Status.PreviousServiceCredentialBindings = newPrevSCBs
		cr.SetConditions(xpv1.Available().WithMessage(msgUpToDate))
		return managed.ExternalUpdate{}, nil
	case msgConnOutdated:
		secret, err := rcb.GetSecret(ctx, c.kube, cr)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				cr.SetConditions(xpv1.Available().WithMessage("waiting for secret to be created"))
				return managed.ExternalUpdate{
					ConnectionDetails: nil,
				}, nil
			}
			cr.SetConditions(xpv1.Unavailable().WithMessage("cannot update copy secret"))
			return managed.ExternalUpdate{}, errors.Wrap(err, "cannot update copy secret")
		}
		cr.SetConditions(xpv1.Available().WithMessage(msgUpToDate))
		return managed.ExternalUpdate{
			ConnectionDetails: secret.Data,
		}, nil
	case msgPrevNotFound:
		if len(cr.Status.PreviousServiceCredentialBindings) == 0 {
			cr.SetConditions(xpv1.Available().WithMessage(msgUpToDate))
			return managed.ExternalUpdate{}, nil
		}
		// If the previous service credential binding is not found, remove it from the list
		newPrevSCBs := make([]*v1alpha1.ServiceCredentialBindingReference, 0, len(cr.Status.PreviousServiceCredentialBindings))
		for _, prevSCB := range cr.Status.PreviousServiceCredentialBindings {
			if err := c.kube.Get(ctx, k8s.ObjectKey{Namespace: prevSCB.Namespace, Name: prevSCB.Name}, &v1alpha1.ServiceCredentialBinding{}); err != nil {
				if k8serrors.IsNotFound(err) {
					continue
				}
				cr.SetConditions(xpv1.Unavailable().WithMessage("cannot get previous service credential binding"))
				return managed.ExternalUpdate{}, errors.Wrap(err, "cannot get previous service credential binding")
			}
			newPrevSCBs = append(newPrevSCBs, prevSCB)
		}
		cr.Status.PreviousServiceCredentialBindings = newPrevSCBs
		return managed.ExternalUpdate{}, nil
	case msgWaitingForSecret:
		if cr.Spec.WriteConnectionSecretToReference == nil || cr.Spec.WriteConnectionSecretToReference.Name == "" {
			cr.SetConditions(xpv1.Available().WithMessage(msgUpToDate))
			return managed.ExternalUpdate{}, nil
		}
		// If the secret is not created yet, we just return and wait for the next reconciliation
		var sourceSecret apicorev1.Secret
		if err := c.kube.Get(ctx, k8s.ObjectKey{Namespace: cr.Status.ActiveServiceCredentialBinding.Namespace, Name: cr.Status.ActiveServiceCredentialBinding.Name}, &sourceSecret); err != nil {
			if k8serrors.IsNotFound(err) {
				cr.SetConditions(xpv1.Available().WithMessage(msgWaitingForSecret))
				return managed.ExternalUpdate{
					ConnectionDetails: nil,
				}, nil
			}
			cr.SetConditions(xpv1.Unavailable().WithMessage("cannot get source secret for current binding"))
			return managed.ExternalUpdate{}, errors.Wrap(err, "cannot get source secret for current binding")
		}
		if len(sourceSecret.Data) == 0 {
			cr.SetConditions(xpv1.Available().WithMessage(msgWaitingForSecret))
			return managed.ExternalUpdate{
				ConnectionDetails: nil,
			}, nil
		}
		// If the secret is created, we can just return and wait for the next reconciliation
		cr.SetConditions(xpv1.Available().WithMessage(msgUpToDate))
		return managed.ExternalUpdate{
			ConnectionDetails: sourceSecret.Data,
		}, nil
	}

	if msg := cr.GetCondition(xpv1.Unavailable().Type).Message; msg == msgSCBNotFound {
		newName, err := rcb.CreateSCB(ctx, c.kube, cr, cr.Status.ActiveServiceCredentialBinding.Name, cr.Status.ActiveServiceCredentialBinding.Namespace)
		if err != nil {
			cr.SetConditions(xpv1.Unavailable().WithMessage("cannot create service credential binding"))
			return managed.ExternalUpdate{}, errors.Wrap(err, "cannot create service credential binding")
		}
		cr.Status.ActiveServiceCredentialBinding = &v1alpha1.ServiceCredentialBindingReference{
			Name:         newName,
			Namespace:    namespace,
			LastRotation: metav1.Time{Time: time.Now()},
		}
		cr.SetConditions(xpv1.Available().WithMessage(msgUpToDate))
		return managed.ExternalUpdate{}, nil
	}

	return managed.ExternalUpdate{}, errors.New("unknown condition message")
}

// Delete a RotatingCredentialBinding resource.
func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.RotatingCredentialBinding)
	if !ok {
		return errors.New(errWrongCRType)
	}

	for _, scb := range append(cr.Status.PreviousServiceCredentialBindings, cr.Status.ActiveServiceCredentialBinding) {
		if scb != nil {
			if err := rcb.DeleteSCB(ctx, c.kube, scb.Name, scb.Namespace); err != nil && !k8serrors.IsNotFound(err) {
				cr.SetConditions(xpv1.Unavailable().WithMessage("cannot delete service credential binding"))
				return errors.Wrap(err, "cannot delete service credential binding")
			}
		}
	}

	cr.SetConditions(xpv1.Deleting().WithMessage("deleted service credential bindings"))

	return nil
}
