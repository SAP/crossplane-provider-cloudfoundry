package clients

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestExtractKey(t *testing.T) {
	secret := &corev1.Secret{
		Data: map[string][]byte{
			"secretKey": []byte("secretValue"),
		},
	}

	value := extractKey(secret, "secretKey")
	assert.Equal(t, []byte("secretValue"), value)

	value = extractKey(secret, "nonexistent")
	assert.Nil(t, value)
}

func TestMarshalSecretData(t *testing.T) {
	data := map[string][]byte{
		"key1": []byte(`{"nestedKey":"nestedValue"}`),
		"key2": []byte("plainValue"),
	}

	result, err := marshalSecretData(data)
	require.NoError(t, err)

	var resultMap map[string]interface{}
	err = json.Unmarshal(result, &resultMap)
	require.NoError(t, err)

	assert.Equal(t, map[string]interface{}{
		"key1": map[string]interface{}{"nestedKey": "nestedValue"},
		"key2": "plainValue",
	}, resultMap)
}

func TestTryUnmarshal(t *testing.T) {
	jsonData := []byte(`{"key":"value"}`)
	plainData := []byte("plainValue")

	parsed := tryUnmarshal(jsonData)
	assert.Equal(t, map[string]interface{}{"key": "value"}, parsed)

	parsed = tryUnmarshal(plainData)
	assert.Equal(t, "plainValue", parsed)
}
