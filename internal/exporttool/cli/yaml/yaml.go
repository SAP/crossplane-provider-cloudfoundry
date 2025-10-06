package yaml

import "sigs.k8s.io/yaml"

func Marshal(resource any) (string, error) {
	b, err := yaml.Marshal(resource)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
