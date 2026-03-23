// Package app implements Cloud Foundry App resource export functionality.
package app

import (
	"github.com/SAP/xp-clifford/yaml"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// generateDockerCredentialSecret creates a Kubernetes Secret for Docker registry credentials.
// The password field is set to "TODO" as a placeholder since the actual password cannot be exported.
// The secret includes a comment reminding users to manually fill in the password.
func generateDockerCredentialSecret(name, username string) *yaml.ResourceWithComment {
	s := &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		StringData: map[string]string{
			"username": username,
			"password": "TODO",
		},
		Type: v1.SecretTypeOpaque,
	}
	s.SetName(name)
	commentedSecret := yaml.NewResourceWithComment(s)
	commentedSecret.AddComment("Cannot export the password value. Fill in the password field manually.")
	return commentedSecret
}
