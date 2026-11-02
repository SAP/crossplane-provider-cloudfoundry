package test

import (
	"fmt"
	"os"
	"regexp"

	"sigs.k8s.io/e2e-framework/support/kind"
)

const kindNodeImageTagEnv = "KIND_NODE_IMAGE_TAG"

var kindNodeImageTagPattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)

// NewKindClusterFromEnvironment configures a kind cluster with the optional
// node image selected by KIND_NODE_IMAGE_TAG. An unset or empty value leaves
// kind's default image selection unchanged. The variable is an image tag, not
// a complete image reference.
func NewKindClusterFromEnvironment() (*kind.Cluster, error) {
	cluster := &kind.Cluster{}
	tag := os.Getenv(kindNodeImageTagEnv)
	if tag == "" {
		return cluster, nil
	}
	if !kindNodeImageTagPattern.MatchString(tag) {
		return nil, fmt.Errorf("%s must be a valid container image tag (1-128 letters, digits, underscores, dots, or hyphens)", kindNodeImageTagEnv)
	}

	cluster.WithOpts(kind.WithImage("kindest/node:" + tag))
	return cluster, nil
}
