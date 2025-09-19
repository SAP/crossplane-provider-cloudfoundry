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

	secret, err := fetchSecret(ctx, kube, sr)
	if err != nil {
		return nil, err
	}

	if key != "" {
		return extractKey(secret, key)
	}
	return marshalSecretData(secret.Data)
}

// fetchSecret retrieves a Kubernetes Secret based on the provided SecretReference.
// It uses Kubernetes client to fetch the Secret from the specified namespace and name.
//
// Parameters:
//   - ctx: The context.
//   - kube: The Kubernetes client.
//   - sr: A reference to the Secret, containing its namespace and name.
//
// Returns:
//   - *v1.Secret: The retrieved Secret object.
//   - error: An error if the Secret could not be fetched or does not exist.
func fetchSecret(ctx context.Context, kube k8s.Client, sr *xpv1.SecretReference) (*v1.Secret, error) {
	secret := &v1.Secret{}
	err := kube.Get(ctx, types.NamespacedName{Namespace: sr.Namespace, Name: sr.Name}, secret)
	return secret, err
}

// extractKey retrieves the value associated with the specified key from the given Kubernetes Secret.
//
// Parameters:
//   - secret: A pointer to a Kubernetes Secret object containing the data.
//   - key: The key to look up in the Secret's data map.
//
// Returns:
//   - []byte: The value associated with the key, if it exists.
//   - error: Always returns nil as no error handling is implemented for missing keys.
func extractKey(secret *v1.Secret, key string) ([]byte, error) {
	if v, ok := secret.Data[key]; ok {
		return v, nil
	}
	return nil, nil
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
		if parsedValue, err := tryUnmarshal(v); err == nil {
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
//   - An error if unmarshaling encounters an issue.
func tryUnmarshal(value []byte) (interface{}, error) {
	var jsonValue interface{}
	if err := json.Unmarshal(value, &jsonValue); err == nil {
		return jsonValue, nil
	}

	var strValue string
	if err := json.Unmarshal(value, &strValue); err == nil {
		if err := json.Unmarshal([]byte(strValue), &jsonValue); err == nil {
			return jsonValue, nil
		}
		return strValue, nil
	}

	return string(value), nil
}
