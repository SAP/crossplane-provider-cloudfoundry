package clients

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestExtractKey(t *testing.T) {
	secret := &corev1.Secret{
		Data: map[string][]byte{
			"secretKey": []byte("secretValue"),
		},
	}

	value, err := extractKey(secret, "secretKey")
	assert.NoError(t, err)
	assert.Equal(t, []byte("secretValue"), value)

	value, err = extractKey(secret, "nonexistent")
	assert.NoError(t, err)
	assert.Nil(t, value)
}

func TestMarshalSecretData(t *testing.T) {
	data := map[string][]byte{
		"key1": []byte(`{"nestedKey":"nestedValue"}`),
		"key2": []byte("plainValue"),
	}

	result, err := marshalSecretData(data)
	assert.NoError(t, err)

	var resultMap map[string]interface{}
	err = json.Unmarshal(result, &resultMap)
	assert.NoError(t, err)

	assert.Equal(t, map[string]interface{}{
		"key1": map[string]interface{}{"nestedKey": "nestedValue"},
		"key2": "plainValue",
	}, resultMap)
}

func TestTryUnmarshal(t *testing.T) {
	jsonData := []byte(`{"key":"value"}`)
	plainData := []byte("plainValue")

	parsed, err := tryUnmarshal(jsonData)
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"key": "value"}, parsed)

	parsed, err = tryUnmarshal(plainData)
	assert.NoError(t, err)
	assert.Equal(t, "plainValue", parsed)
}
