package test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func TestRotationPublicationWaitsForMatchingReadback(t *testing.T) {
	snapshot := validRotationPublicationSnapshot()
	stale := snapshot
	stale.Secret = snapshot.Secret.DeepCopy()
	stale.Secret.Data["username"] = []byte("old")
	reads := 0
	err := WaitForServiceCredentialBindingPublication(context.Background(), time.Second, time.Millisecond, "binding-k1", true, func(context.Context) (ServiceCredentialBindingPublicationSnapshot, error) {
		reads++
		if reads == 1 {
			return stale, nil
		}
		return snapshot, nil
	})
	if err != nil {
		t.Fatal("wait for current credentials:", err)
	}
	if reads < 2 {
		t.Fatalf("publication was accepted before matching readback; reads=%d", reads)
	}
}

func TestRotationPublicationRetriesTransientReadFailure(t *testing.T) {
	snapshot := validRotationPublicationSnapshot()
	reads := 0
	err := WaitForServiceCredentialBindingPublication(context.Background(), time.Second, time.Millisecond, "binding-k1", true, func(context.Context) (ServiceCredentialBindingPublicationSnapshot, error) {
		reads++
		if reads == 1 {
			return ServiceCredentialBindingPublicationSnapshot{}, errors.New("temporary read failure")
		}
		return snapshot, nil
	})
	if err != nil {
		t.Fatal("wait for publication after transient read failure:", err)
	}
	if reads < 2 {
		t.Fatalf("publication was accepted without retrying the failed read; reads=%d", reads)
	}
}

func TestRotationPublicationPredicate(t *testing.T) {
	base := validRotationPublicationSnapshot()
	cases := []struct {
		name   string
		mutate func(*ServiceCredentialBindingPublicationSnapshot)
		want   bool
	}{
		{name: "matching data with ancillary owner", want: true},
		{name: "wrong generation", mutate: func(s *ServiceCredentialBindingPublicationSnapshot) { s.BindingGUID = "binding-k0" }},
		{name: "details retrieval failed", mutate: func(s *ServiceCredentialBindingPublicationSnapshot) { s.ExpectedDetails = nil }},
		{name: "secret missing", mutate: func(s *ServiceCredentialBindingPublicationSnapshot) { s.Secret = nil }},
		{name: "wrong Secret reference", mutate: func(s *ServiceCredentialBindingPublicationSnapshot) { s.Secret.Name = "other" }},
		{name: "wrong Secret type", mutate: func(s *ServiceCredentialBindingPublicationSnapshot) { s.Secret.Type = corev1.SecretTypeOpaque }},
		{name: "missing data", mutate: func(s *ServiceCredentialBindingPublicationSnapshot) { delete(s.Secret.Data, "username") }},
		{name: "stale data", mutate: func(s *ServiceCredentialBindingPublicationSnapshot) { s.Secret.Data["username"] = []byte("old") }},
		{name: "stale data with newer resourceVersion", mutate: func(s *ServiceCredentialBindingPublicationSnapshot) {
			s.Secret.ResourceVersion = "999"
			s.Secret.Data["username"] = []byte("old")
		}},
		{name: "foreign controller", mutate: func(s *ServiceCredentialBindingPublicationSnapshot) {
			for i := range s.Secret.OwnerReferences {
				if s.Secret.OwnerReferences[i].Controller != nil && *s.Secret.OwnerReferences[i].Controller {
					s.Secret.OwnerReferences[i].UID = types.UID("foreign")
				}
			}
		}},
		{name: "no controller", mutate: func(s *ServiceCredentialBindingPublicationSnapshot) {
			s.Secret.OwnerReferences = s.Secret.OwnerReferences[:1]
		}},
		{name: "empty details accepted when allowed", mutate: func(s *ServiceCredentialBindingPublicationSnapshot) { s.ExpectedDetails = map[string][]byte{} }, want: true},
		{name: "empty details rejected for broker fixture", mutate: func(s *ServiceCredentialBindingPublicationSnapshot) { s.ExpectedDetails = map[string][]byte{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := cloneRotationPublicationSnapshot(base)
			if tc.mutate != nil {
				tc.mutate(&snapshot)
			}
			requireNonEmpty := !strings.Contains(tc.name, "accepted when allowed")
			if got := serviceCredentialBindingPublicationMatches(snapshot, "binding-k1", requireNonEmpty); got != tc.want {
				t.Fatalf("publication match = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestRotationCredentialChangeRequiresDifferentNonEmptyClientSecret(t *testing.T) {
	const sentinel = "credential-sentinel-must-not-appear"
	cases := []struct {
		name    string
		old     map[string]any
		current map[string]any
		wantErr bool
	}{
		{name: "different secrets", old: map[string]any{"clientsecret": "old"}, current: map[string]any{"clientsecret": "new"}},
		{name: "same secret despite representation change", old: map[string]any{"clientsecret": "same", "old-json": "value"}, current: map[string]any{"clientsecret": "same"}, wantErr: true},
		{name: "empty old secret", old: map[string]any{"clientsecret": ""}, current: map[string]any{"clientsecret": "new"}, wantErr: true},
		{name: "missing old secret", old: map[string]any{}, current: map[string]any{"clientsecret": "new"}, wantErr: true},
		{name: "non-string old secret", old: map[string]any{"clientsecret": 42}, current: map[string]any{"clientsecret": "new"}, wantErr: true},
		{name: "empty current secret", old: map[string]any{"clientsecret": "old"}, current: map[string]any{"clientsecret": ""}, wantErr: true},
		{name: "missing current secret", old: map[string]any{"clientsecret": "old"}, current: map[string]any{}, wantErr: true},
		{name: "non-string current secret", old: map[string]any{"clientsecret": "old"}, current: map[string]any{"clientsecret": 42}, wantErr: true},
		{name: "credential values are not included in errors", old: map[string]any{"clientsecret": sentinel}, current: map[string]any{"clientsecret": sentinel}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRotationCredentialChange(tc.old, tc.current)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validation error = %v, wantErr %t", err, tc.wantErr)
			}
			if err != nil && strings.Contains(err.Error(), sentinel) {
				t.Fatal("credential value leaked into validation error")
			}
		})
	}
}

func TestRotationPublicationWaitBoundsAndSanitizesErrors(t *testing.T) {
	sentinel := errors.New("credential-value-must-not-leak")
	err := WaitForServiceCredentialBindingPublication(context.Background(), 10*time.Millisecond, time.Millisecond, "binding-k1", true, func(context.Context) (ServiceCredentialBindingPublicationSnapshot, error) {
		return ServiceCredentialBindingPublicationSnapshot{}, sentinel
	})
	if err == nil || strings.Contains(err.Error(), sentinel.Error()) || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("unexpected sanitized timeout error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = WaitForServiceCredentialBindingPublication(ctx, time.Second, time.Millisecond, "binding-k1", true, func(context.Context) (ServiceCredentialBindingPublicationSnapshot, error) {
		t.Fatal("read callback must not run after cancellation")
		return ServiceCredentialBindingPublicationSnapshot{}, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func validRotationPublicationSnapshot() ServiceCredentialBindingPublicationSnapshot {
	controller := metav1.OwnerReference{APIVersion: "cloudfoundry.crossplane.io/v1alpha1", Kind: "ServiceCredentialBinding", Name: "binding", UID: types.UID("owner"), Controller: ptr.To(true)}
	ancillary := metav1.OwnerReference{APIVersion: "v1", Kind: "ConfigMap", Name: "extra", UID: types.UID("extra"), Controller: ptr.To(false)}
	return ServiceCredentialBindingPublicationSnapshot{
		BindingGUID:     "binding-k1",
		SecretNamespace: "default",
		SecretName:      "credentials",
		ExpectedDetails: map[string][]byte{"username": []byte("user")},
		Secret:          &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "credentials", Namespace: "default", OwnerReferences: []metav1.OwnerReference{ancillary, controller}}, Type: corev1.SecretType("connection.crossplane.io/v1alpha1"), Data: map[string][]byte{"username": []byte("user"), "old": []byte("retained")}},
		ExpectedOwner:   &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "credentials", Namespace: "default", OwnerReferences: []metav1.OwnerReference{controller}}, Type: corev1.SecretType("connection.crossplane.io/v1alpha1")},
	}
}

func cloneRotationPublicationSnapshot(in ServiceCredentialBindingPublicationSnapshot) ServiceCredentialBindingPublicationSnapshot {
	out := in
	if in.Secret != nil {
		out.Secret = in.Secret.DeepCopy()
	}
	if in.ExpectedOwner != nil {
		out.ExpectedOwner = in.ExpectedOwner.DeepCopy()
	}
	if in.ExpectedDetails != nil {
		out.ExpectedDetails = make(map[string][]byte, len(in.ExpectedDetails))
		for key, value := range in.ExpectedDetails {
			out.ExpectedDetails[key] = append([]byte(nil), value...)
		}
	}
	return out
}
