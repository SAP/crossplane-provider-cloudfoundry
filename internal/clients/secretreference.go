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

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
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

	// if key is specified, return the value of the key
	if key != "" {
		if v, ok := secret.Data[key]; ok {
			return v, nil
		}
		return nil, nil
	}

	// if key is not specified, return all data from the secret, also string or nested JSON
	data := make(map[string]interface{})
	for k, v := range secret.Data {
		// Try to parse as JSON first
		var jsonValue interface{}
		if err := json.Unmarshal(v, &jsonValue); err == nil {
			data[k] = jsonValue
		} else {
			// If not JSON, store as string
			data[k] = string(v)
		}
	}

	return json.Marshal(data)
}
