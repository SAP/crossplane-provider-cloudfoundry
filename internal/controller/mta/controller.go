/*
Copyright 2022 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mta

import (
	"context"
	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	apisv1alpha1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1alpha1"
	pcv1beta1 "github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/mta"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/features"
	mtaClient "github.com/cloudfoundry-incubator/multiapps-cli-plugin/clients/mtaclient"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

const (
	errNotMta       = "managed resource is not a Mta custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"

	errNewClient = "cannot create new Client"

	errGetSecret  = "cannot get Secret"
	errGet        = "cannot get MTA"
	errCreate     = "cannot create MTA"
	errCreateFile = "cannot create MTA file"

	errCreateMtaExt = "cannot create MTA extension"

	errUpdateCR = "cannot update the managed resource"
	errDelete   = "cannot delete MTA"
)

// A NoOpService does nothing.
type NoOpService struct{}

var (
	newNoOpService = func(_ []byte) (interface{}, error) { return &NoOpService{}, nil }
)

// Setup adds a controller that reconciles Mta managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.MtaGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.MtaGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &pcv1beta1.ProviderConfigUsage{}),
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.Mta{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
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
	cr, ok := mg.(*v1alpha1.Mta)
	if !ok {
		return nil, errors.New(errNotMta)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	client, err := clients.ClientFnBuilderMta(ctx, c.kube, cr.Spec.ForProvider.Space)(mg)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{kube: c.kube, mta: *client}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	// A 'client' used to connect to the external resource API. In practice this
	// would be something like an AWS SDK client.
	kube k8s.Client
	mta  mtaClient.MtaClientOperations
}

func (c *external) Disconnect(ctx context.Context) error {
	return nil
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Mta)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotMta)
	}

	if cr.Status.AtProvider.MtaId == nil && cr.Status.AtProvider.LastOperation == nil && cr.Status.AtProvider.Files == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	observation, err := mta.Observe(cr, c.mta)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGet)
	}

	cr.Status.AtProvider = observation
	if err = c.kube.Status().Update(ctx, cr); err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errUpdateCR)
	}

	if cr.HasErrorOperation() {
		return managed.ExternalObservation{}, errors.New(cr.GetErrorOperation())
	}
	if cr.HasRunningOperation() {
		return managed.ExternalObservation{ResourceExists: true}, nil
	}
	if exists, err := mta.Exists(cr, c.mta); !exists || cr.Status.AtProvider.LastOperation == nil {
		return managed.ExternalObservation{ResourceExists: false}, err
	}
	if cr.HasChangedUrls() || cr.HasExtensionChanged() {
		// files can't be reused, so we need to recreate them
		cr.Status.AtProvider.Files = nil
		cr.Status.AtProvider.MtaExtensionId = nil
		cr.Status.AtProvider.MtaExtensionHash = nil
		cr.Status.AtProvider.LastOperation = nil
		if err = c.kube.Status().Update(ctx, cr); err != nil {
			return managed.ExternalObservation{}, errors.Wrap(err, errUpdateCR)
		}

		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Mta)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotMta)
	}

	cr.SetConditions(xpv1.Creating())

	observation, err := mta.Deploy(cr, c.mta)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreate)
	}

	fileObservations, err := c.createFiles(ctx, cr)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateFile)
	}
	observation.Files = &fileObservations

	err = mta.CreateExtensions(cr, &observation, c.mta)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateMtaExt)
	}

	cr.Status.AtProvider = observation
	if err = c.kube.Status().Update(ctx, cr); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errUpdateCR)
	}

	return managed.ExternalCreation{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	// MTA can not be updated
	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Mta)
	if !ok {
		return errors.New(errNotMta)
	}

	cr.SetConditions(xpv1.Deleting())

	observation, err := mta.Delete(cr, c.mta)
	if err != nil {
		return errors.Wrap(err, errDelete)
	}

	cr.Status.AtProvider = observation
	if err = c.kube.Status().Update(ctx, cr); err != nil {
		return errors.Wrap(err, errUpdateCR)
	}

	return nil
}

func (c *external) createFiles(ctx context.Context, cr *v1alpha1.Mta) ([]v1alpha1.FileObservation, error) {
	fileObservations := []v1alpha1.FileObservation{}
	for _, file := range cr.AllFiles() {
		fileObservation := cr.FindFileObservation(&file)

		if fileObservation == nil {
			var secret *v1.Secret
			if file.CredentialsSecretRef != nil {
				secret = &v1.Secret{}
				err := c.kube.Get(ctx, types.NamespacedName{Name: file.CredentialsSecretRef.Name, Namespace: file.CredentialsSecretRef.Namespace}, secret)
				if err != nil {
					return fileObservations, errors.Wrap(err, errGetSecret)
				}
			}

			o, err := mta.UploadFileFromUrl(cr, &file, secret, c.mta)
			if err != nil {
				return fileObservations, errors.Wrap(err, errCreate)
			}
			fileObservation = &o
		}

		fileObservations = append(fileObservations, *fileObservation)
	}

	return fileObservations, nil
}
