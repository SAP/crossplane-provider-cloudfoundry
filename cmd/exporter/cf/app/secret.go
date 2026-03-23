package app

import (
	"github.com/SAP/xp-clifford/yaml"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
