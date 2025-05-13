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

// SecretRefToJSONRawMessage extracts parameters/credentials from a secret reference.
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

	// if key is not specified, return all data from the secret
	cred := make(map[string]string)
	for k, v := range secret.Data {
		cred[k] = string(v)
	}
	buf, err := json.Marshal(cred)
	if err != nil {
		return nil, err
	}
	return buf, nil
}
