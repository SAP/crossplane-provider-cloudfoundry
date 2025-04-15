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
func SecretRefToJSONRawMessage(ctx context.Context, kube k8s.Client, sr *xpv1.SecretReference) (json.RawMessage, error) {
	if sr == nil {
		return nil, nil
	}

	secret := &v1.Secret{}
	if err := kube.Get(ctx, types.NamespacedName{Namespace: sr.Namespace, Name: sr.Name}, secret); err != nil {
		return nil, err
	}

	// if key is not specified, return all data from the secret
	cred := make(map[string]string)
	for key, value := range secret.Data {
		cred[key] = string(value)
	}
	buf, err := json.Marshal(cred)
	if err != nil {
		return nil, err
	}
	return buf, nil
}
