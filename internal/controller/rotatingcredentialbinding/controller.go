package rotatingcredentialbinding

import (
	"context"
	"time"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/pkg/errors"
	apicorev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	rcb "github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/rotatingcredentialbinding"

	ctrl "sigs.k8s.io/controller-runtime"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	apisv1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1alpha1"
	apisv1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/features"
)

const (
	errTrackPCUsage       = "cannot track ProviderConfig usage"
	errWrongCRType        = "managed resource is not a RotatingCredentialBinding"
	errCannotUpdateStatus = "cannot update status"

	msgUpToDate              = "credentials are up to date"
	msgRotationDue           = "credentials are due for rotation"
	msgDeleteExpired         = "expired credentials have to be deleted"
	msgSCBNotFound           = "service credential binding not found"
	msgCannotGetSCB          = "cannot get active service credential binding"
	msgCannotGetSourceSecret = "cannot get source secret for current service binding"
	msgCannotCreateSCB       = "cannot create service credential binding"
	msgCannotListRetiredSCBs = "cannot list retired service credential bindings"
	msgCannotDeleteSCBs      = "cannot delete expired service credential bindings"

	ForceRotationKey = "rotatingcredentialbinding.cloudfoundry.crossplane.io/force-rotation"
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
		cr.SetConditions(xpv1.Creating().WithMessage(msgSCBNotFound))
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	name := cr.Status.ActiveServiceCredentialBinding.Name
	namespace := cr.Status.ActiveServiceCredentialBinding.Namespace

	var activeSCB v1alpha1.ServiceCredentialBinding
	if err := c.kube.Get(ctx, k8s.ObjectKey{Namespace: namespace, Name: name}, &activeSCB); err != nil {
		if k8serrors.IsNotFound(err) {
			cr.SetConditions(xpv1.Unavailable().WithMessage(msgSCBNotFound))
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		cr.SetConditions(xpv1.Unavailable().WithMessage(msgCannotGetSCB))
		return managed.ExternalObservation{}, errors.Wrap(err, msgCannotGetSCB)
	}

	if err := c.updateExternalName(ctx, cr, &activeSCB); err != nil {
		return managed.ExternalObservation{}, err
	}

	if activeSCB.GetCreationTimestamp().Add(cr.Spec.RotationFrequency.Duration).Before(time.Now()) {
		cr.SetConditions(xpv1.Available().WithMessage(msgRotationDue))
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	if checkForceRotation(cr) {
		cr.SetConditions(xpv1.Available().WithMessage(msgRotationDue))
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	expired, err := c.checkSCBExpired(ctx, cr, &activeSCB)
	if err != nil {
		cr.SetConditions(xpv1.Unavailable().WithMessage(msgCannotListRetiredSCBs))
		return managed.ExternalObservation{ResourceExists: true}, err
	}

	if expired {
		cr.SetConditions(xpv1.Available().WithMessage(msgDeleteExpired))
		return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
	}

	return c.checkSecretUpdated(ctx, cr, &activeSCB)
}

// Create a RotatingCredentialBinding resource.
func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.RotatingCredentialBinding)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errWrongCRType)
	}

	cr.SetConditions(xpv1.Creating())

	name := cr.Spec.ForProvider.Name
	namespace := getNamespace(cr)

	newName, err := rcb.GenerateSCB(ctx, c.kube, cr, name, namespace)
	if err != nil {
		cr.SetConditions(xpv1.Unavailable().WithMessage(msgCannotCreateSCB))
		return managed.ExternalCreation{}, errors.Wrap(err, msgCannotCreateSCB)
	}

	cr.Status.ActiveServiceCredentialBinding = &v1alpha1.ServiceCredentialBindingReference{
		Name:      newName,
		Namespace: namespace,
	}
	cr.SetConditions(xpv1.Available().WithMessage(msgUpToDate))

	if err := c.kube.Status().Update(ctx, cr); err != nil {
		cr.SetConditions(xpv1.Unavailable().WithMessage(errCannotUpdateStatus))
		return managed.ExternalCreation{}, errors.Wrap(err, errCannotUpdateStatus)
	}

	if cr.ObjectMeta.Annotations != nil {
		if _, ok := cr.ObjectMeta.Annotations[ForceRotationKey]; ok {
			meta.RemoveAnnotations(cr, ForceRotationKey)
		}
	}

	return managed.ExternalCreation{}, nil
}

// Update a RotatingCredentialBinding resource.
func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.RotatingCredentialBinding)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errWrongCRType)
	}

	allSCBs, err := rcb.GetAllBindings(ctx, c.kube, cr)
	if err != nil {
		cr.SetConditions(xpv1.Unavailable().WithMessage(msgCannotListRetiredSCBs))
		return managed.ExternalUpdate{}, errors.Wrap(err, msgCannotListRetiredSCBs)
	}

	if err := rcb.DeleteSCBs(ctx, c.kube, allSCBs, cr); err != nil {
		cr.SetConditions(xpv1.Unavailable().WithMessage(msgCannotDeleteSCBs))
		return managed.ExternalUpdate{}, errors.Wrap(err, msgCannotDeleteSCBs)
	}

	return managed.ExternalUpdate{}, nil
}

// Delete a RotatingCredentialBinding resource.
func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.RotatingCredentialBinding)
	if !ok {
		return errors.New(errWrongCRType)
	}

	allSCBs, err := rcb.GetAllBindings(ctx, c.kube, cr)
	if err != nil {
		cr.SetConditions(xpv1.Unavailable().WithMessage(msgCannotListRetiredSCBs))
		return errors.Wrap(err, msgCannotListRetiredSCBs)
	}

	if err := rcb.DeleteSCBs(ctx, c.kube, allSCBs, nil); err != nil {
		cr.SetConditions(xpv1.Unavailable().WithMessage(msgCannotDeleteSCBs))
		return errors.Wrap(err, msgCannotDeleteSCBs)
	}

	cr.SetConditions(xpv1.Deleting().WithMessage("deleted service credential bindings"))

	return nil
}

func getNamespace(cr *v1alpha1.RotatingCredentialBinding) string {
	if cr.Spec.ForProvider.Namespace != nil && *cr.Spec.ForProvider.Namespace != "" {
		return *cr.Spec.ForProvider.Namespace
	}
	if cr.GetNamespace() != "" {
		return cr.GetNamespace()
	}
	return "default"
}

func (c *external) checkSecretUpdated(ctx context.Context, cr *v1alpha1.RotatingCredentialBinding, activeSCB *v1alpha1.ServiceCredentialBinding) (managed.ExternalObservation, error) {
	if cr.Spec.WriteConnectionSecretToReference != nil && cr.Spec.WriteConnectionSecretToReference.Name != "" {
		var sourceSecret apicorev1.Secret
		if err := c.kube.Get(ctx,
			k8s.ObjectKey{
				Namespace: activeSCB.Spec.WriteConnectionSecretToReference.Namespace,
				Name:      activeSCB.Spec.WriteConnectionSecretToReference.Name,
			},
			&sourceSecret); err != nil {
			cr.SetConditions(xpv1.Unavailable().WithMessage(msgCannotGetSourceSecret))
			return managed.ExternalObservation{}, errors.Wrap(err, msgCannotGetSourceSecret)
		}

		cr.SetConditions(xpv1.Available().WithMessage(msgUpToDate))
		return managed.ExternalObservation{
			ResourceExists:    true,
			ResourceUpToDate:  true,
			ConnectionDetails: sourceSecret.Data,
		}, nil

	}

	cr.SetConditions(xpv1.Available().WithMessage(msgUpToDate))
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
}

func (c *external) checkSCBExpired(ctx context.Context, cr *v1alpha1.RotatingCredentialBinding, activeSCB *v1alpha1.ServiceCredentialBinding) (bool, error) {
	allSCBs, err := rcb.GetAllBindings(ctx, c.kube, cr)
	if err != nil {
		cr.SetConditions(xpv1.Unavailable().WithMessage(msgCannotListRetiredSCBs))
		return false, errors.Wrap(err, msgCannotListRetiredSCBs)
	}
	for _, currentSCB := range allSCBs {
		if currentSCB.Name == activeSCB.Name && currentSCB.Namespace == activeSCB.Namespace {
			// If the previous SCB is the same as the active one, we do not
			// delete it.
			continue
		}
		if currentSCB.GetCreationTimestamp().Add(cr.Spec.RotationTTL.Duration).Before(time.Now()) {
			cr.SetConditions(xpv1.Available().WithMessage(msgDeleteExpired))
			return true, nil
		}
	}
	return false, nil
}

func checkForceRotation(cr *v1alpha1.RotatingCredentialBinding) bool {
	if cr.ObjectMeta.Annotations != nil {
		if _, ok := cr.ObjectMeta.Annotations[ForceRotationKey]; ok {
			cr.SetConditions(xpv1.Available().WithMessage(msgRotationDue))
			return true
		}
	}
	return false
}

func (c *external) updateExternalName(ctx context.Context, cr *v1alpha1.RotatingCredentialBinding, activeSCB *v1alpha1.ServiceCredentialBinding) error {
	if meta.GetExternalName(cr) != meta.GetExternalName(activeSCB) {
		meta.SetExternalName(cr, meta.GetExternalName(activeSCB))
		if err := c.kube.Update(ctx, cr); err != nil {
			cr.SetConditions(xpv1.Unavailable().WithMessage(errCannotUpdateStatus))
			return errors.Wrap(err, errCannotUpdateStatus)
		}
	}
	return nil
}
