package v1alpha1

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestServiceCredentialBindingRetiredStateSerialization(t *testing.T) {
	const legacyJSON = `{"guid":"11111111-1111-4111-8111-111111111111","name":"old-key","createdAt":"2026-01-01T00:00:00Z"}`
	var legacy SCBResource
	require.NoError(t, json.Unmarshal([]byte(legacyJSON), &legacy))
	require.Equal(t, "11111111-1111-4111-8111-111111111111", legacy.GUID)
	require.Nil(t, legacy.DeleteAfter, "legacy serialized records remain valid without the optional deadline")

	deadline := metav1.NewTime(time.Date(2026, time.October, 30, 9, 30, 0, 0, time.UTC))
	retired := SCBResource{GUID: legacy.GUID, Name: legacy.Name, CreatedAt: legacy.CreatedAt, DeleteAfter: &deadline}
	encoded, err := json.Marshal(retired)
	require.NoError(t, err)
	var decoded SCBResource
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.True(t, deadline.Time.Equal(decoded.DeleteAfter.Time))

	resource := &ServiceCredentialBinding{Status: ServiceCredentialBindingStatus{AtProvider: ServiceCredentialBindingObservation{
		RetiredKeys: []*SCBResource{&retired},
	}}}
	copied := resource.DeepCopy()
	*copied.Status.AtProvider.RetiredKeys[0].DeleteAfter = metav1.NewTime(deadline.Add(time.Hour))
	require.Equal(t, deadline, *resource.Status.AtProvider.RetiredKeys[0].DeleteAfter, "deepcopy must own the optional timestamp")
}
