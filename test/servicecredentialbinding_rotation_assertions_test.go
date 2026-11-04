package test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

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

func TestRotationRetiredKeyCollectionWaitsForBothStores(t *testing.T) {
	for _, tc := range []struct {
		name        string
		cfReads     []bool
		statusReads [][]*v1alpha1.SCBResource
	}{
		{name: "both absent", cfReads: []bool{false}, statusReads: [][]*v1alpha1.SCBResource{nil}},
		{name: "external key initially present", cfReads: []bool{true, false}, statusReads: [][]*v1alpha1.SCBResource{nil, nil}},
		{name: "status entry initially present", cfReads: []bool{false, false}, statusReads: [][]*v1alpha1.SCBResource{{{GUID: "k0"}}, nil}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfIndex, statusIndex := 0, 0
			err := WaitForRotationRetiredKeyCollection(context.Background(), time.Second, time.Millisecond, "k0",
				func(context.Context) (bool, error) {
					index := cfIndex
					if cfIndex < len(tc.cfReads)-1 {
						cfIndex++
					}
					return tc.cfReads[index], nil
				},
				func(context.Context) ([]*v1alpha1.SCBResource, error) {
					index := statusIndex
					if statusIndex < len(tc.statusReads)-1 {
						statusIndex++
					}
					return tc.statusReads[index], nil
				})
			if err != nil {
				t.Fatalf("collection wait failed: %v", err)
			}
		})
	}
}

func TestRotationRetiredKeyCollectionDoesNotTreatReadErrorsAsAbsence(t *testing.T) {
	secretSentinel := errors.New("credential-sentinel-must-not-leak")
	err := WaitForRotationRetiredKeyCollection(context.Background(), time.Second, time.Millisecond, "k0",
		func(context.Context) (bool, error) { return false, secretSentinel },
		func(context.Context) ([]*v1alpha1.SCBResource, error) { return nil, nil })
	if err == nil || strings.Contains(err.Error(), secretSentinel.Error()) {
		t.Fatalf("external read failure was not safely reported: %v", err)
	}
	err = WaitForRotationRetiredKeyCollection(context.Background(), time.Second, time.Millisecond, "k0",
		func(context.Context) (bool, error) { return false, nil },
		func(context.Context) ([]*v1alpha1.SCBResource, error) { return nil, secretSentinel })
	if err == nil || strings.Contains(err.Error(), secretSentinel.Error()) {
		t.Fatalf("status read failure was not safely reported: %v", err)
	}
}

func TestRotationActiveBindingUsableRequiresIdentityOperationAndCFPresence(t *testing.T) {
	for _, tc := range []struct {
		name      string
		guid      string
		succeeded bool
		present   bool
		want      bool
	}{
		{name: "usable active key", guid: "k1", succeeded: true, present: true, want: true},
		{name: "missing identity", succeeded: true, present: true},
		{name: "operation incomplete", guid: "k1", present: true},
		{name: "CF key absent", guid: "k1", succeeded: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := RotationActiveBindingUsable(tc.guid, tc.succeeded, tc.present); got != tc.want {
				t.Fatalf("active usability = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestRotationRetiredKeyCollectedRequiresExternalAndStatusAbsence(t *testing.T) {
	cases := []struct {
		name      string
		guid      string
		cfPresent bool
		retired   []*v1alpha1.SCBResource
		want      bool
	}{
		{name: "both absent", guid: "k0", want: true},
		{name: "CF remains", guid: "k0", cfPresent: true},
		{name: "status remains", guid: "k0", retired: []*v1alpha1.SCBResource{{GUID: "k0"}}},
		{name: "nil status entry ignored", guid: "k0", retired: []*v1alpha1.SCBResource{nil, {GUID: "k1"}}, want: true},
		{name: "nil and empty status", guid: "k0", retired: []*v1alpha1.SCBResource{nil}, want: true},
		{name: "active identity absent is not inferred", guid: "", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RotationRetiredKeyCollected(tc.guid, tc.cfPresent, tc.retired); got != tc.want {
				t.Fatalf("collection predicate = %t, want %t", got, tc.want)
			}
		})
	}

}
