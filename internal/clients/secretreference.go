/*
Copyright 2023 SAP SE
*/

package clients

import (
	"context"
	"encoding/json"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	k8s "sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// ExtractSecret extracts parameters/credentials from a secret reference.
// If a key is specified, returns the raw value for that key.
// If no key is specified, returns all secret data as nested JSON/YAML.
func ExtractSecret(ctx context.Context, kube k8s.Client, sr *xpv1.SecretReference, key string) ([]byte, error) {
	if sr == nil {
		return nil, nil
	}

	secret := &v1.Secret{}
	if err := kube.Get(ctx, types.NamespacedName{Namespace: sr.Namespace, Name: sr.Name}, secret); err != nil {
		return nil, err
	}

	if key != "" {
		return extractKey(secret, key), nil
	}
	return marshalSecretData(secret.Data)
}

// extractKey retrieves the value associated with the specified key from the given Kubernetes Secret.
//
// Parameters:
//   - secret: A pointer to a Kubernetes Secret object containing the data.
//   - key: The key to look up in the Secret's data map.
//
// Returns:
//   - []byte: The value associated with the key, if it exists or returns nil for missing keys.
func extractKey(secret *v1.Secret, key string) ([]byte) {
	if v, ok := secret.Data[key]; ok {
		return v
	}
	return nil
}

// marshalSecretData attempts to marshal data into a JSON-encoded byte slice. For each key-value pair in the input map:
//
// Parameters:
//   - data: A map where keys are strings and values are byte slices representing secret data.
//
// Returns:
//   - A JSON-encoded byte slice representing the processed secret data.
//   - An error if the JSON marshaling fails.
func marshalSecretData(data map[string][]byte) ([]byte, error) {
	result := make(map[string]interface{})
	for k, v := range data {
		if parsedValue := tryUnmarshal(v); parsedValue != nil {
			result[k] = parsedValue
		} else {
			result[k] = string(v)
		}
	}
	return json.Marshal(result)
}

// tryUnmarshal attempts to unmarshal a byte slice into a Go data structure.
//
// Parameters:
//   - value: The byte slice to be unmarshaled.
//
// Returns:
//   - An interface{} representing the unmarshaled value, or the input as a string
//     if unmarshaling fails.
func tryUnmarshal(value []byte) interface{} {
	var jsonValue interface{}
	if err := json.Unmarshal(value, &jsonValue); err == nil {
		return jsonValue
	}

	var strValue string
	if err := json.Unmarshal(value, &strValue); err == nil {
		if err := json.Unmarshal([]byte(strValue), &jsonValue); err == nil {
			return jsonValue
		}
		return strValue
	}

	return string(value)
}
