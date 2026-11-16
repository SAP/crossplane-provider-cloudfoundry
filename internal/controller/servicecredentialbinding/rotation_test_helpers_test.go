package servicecredentialbinding

import (
	"context"
	"errors"

	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

const (
	rotationTestK0 = "11111111-1111-4111-8111-111111111111"
	rotationTestK1 = "22222222-2222-4222-8222-222222222222"
)

// rotationStoredClient models independent API-server state for ServiceCredentialBinding
// lifecycle tests. Metadata Update and status Update preserve the other API subresource.
type rotationStoredClient struct {
	*test.MockClient
	stored             *v1alpha1.ServiceCredentialBinding
	conflictNext       bool
	conflictsRemaining int
	statusUpdateErr    error
	updateErr          error
	calls              []string
}

func newRotationStoredClient(initial *v1alpha1.ServiceCredentialBinding) *rotationStoredClient {
	s := &rotationStoredClient{stored: initial.DeepCopy()}
	c := &test.MockClient{}
	s.MockClient = c

	c.MockGet = func(_ context.Context, _ k8s.ObjectKey, obj k8s.Object) error {
		target, ok := obj.(*v1alpha1.ServiceCredentialBinding)
		if !ok {
			return errors.New("rotationStoredClient: unexpected object type")
		}
		*target = *s.stored.DeepCopy()
		return nil
	}
	c.MockUpdate = func(_ context.Context, obj k8s.Object, _ ...k8s.UpdateOption) error {
		s.calls = append(s.calls, "Update")
		if s.updateErr != nil {
			return s.updateErr
		}
		if s.conflictNext || s.conflictsRemaining > 0 {
			s.conflictNext = false
			if s.conflictsRemaining > 0 {
				s.conflictsRemaining--
			}
			return apierrors.NewConflict(schema.GroupResource{Group: v1alpha1.CRDGroup, Resource: "servicecredentialbindings"}, obj.GetName(), errors.New("scripted conflict"))
		}
		incoming, ok := obj.(*v1alpha1.ServiceCredentialBinding)
		if !ok {
			return errors.New("rotationStoredClient: unexpected object type")
		}
		next := incoming.DeepCopy()
		next.Status = s.stored.Status
		s.stored = next
		// A successful Kubernetes Update decodes the complete server response
		// into the caller, including status preserved by the status subresource.
		*incoming = *next.DeepCopy()
		return nil
	}
	c.MockStatusUpdate = func(_ context.Context, obj k8s.Object, _ ...k8s.SubResourceUpdateOption) error {
		s.calls = append(s.calls, "StatusUpdate")
		if s.statusUpdateErr != nil {
			return s.statusUpdateErr
		}
		incoming, ok := obj.(*v1alpha1.ServiceCredentialBinding)
		if !ok {
			return errors.New("rotationStoredClient: unexpected object type")
		}
		next := s.stored.DeepCopy()
		next.Status = incoming.Status
		s.stored = next
		return nil
	}
	return s
}

func (s *rotationStoredClient) get() *v1alpha1.ServiceCredentialBinding {
	return s.stored.DeepCopy()
}
