package test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/e2e-framework/support/kind"
)

func TestKindNodeImageSelection(t *testing.T) {
	for _, tc := range []struct {
		name string
		tag  string
		want string
	}{
		{name: "empty", tag: ""},
		{name: "default tag", tag: "v1.32.0", want: "kindest/node:v1.32.0"},
		{name: "alternate tag", tag: "v1.31.4", want: "kindest/node:v1.31.4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(kindNodeImageTagEnv, tc.tag)
			assertKindCreateImage(t, tc.want)
		})
	}
}

func TestKindNodeImageUnsetUsesKindDefault(t *testing.T) {
	unsetEnvironmentVariable(t, kindNodeImageTagEnv)
	assertKindCreateImage(t, "")
}

func TestKindNodeImageReadsEnvironmentEachTime(t *testing.T) {
	t.Setenv(kindNodeImageTagEnv, "v1.32.0")
	first, err := NewKindClusterFromEnvironment()
	if err != nil {
		t.Fatalf("NewKindClusterFromEnvironment() error = %v", err)
	}

	t.Setenv(kindNodeImageTagEnv, "v1.31.4")
	second, err := NewKindClusterFromEnvironment()
	if err != nil {
		t.Fatalf("NewKindClusterFromEnvironment() error = %v", err)
	}

	assertKindClusterImage(t, first, "kindest/node:v1.32.0")
	assertKindClusterImage(t, second, "kindest/node:v1.31.4")
}

func TestKindNodeImageRejectsInvalidTags(t *testing.T) {
	for _, tag := range []string{
		" v1.32.0",
		"v1.32.0 ",
		"kindest/node:v1.32.0",
		"v1.32.0;touch /tmp/unexpected",
		strings.Repeat("a", 129),
	} {
		t.Run(tag, func(t *testing.T) {
			t.Setenv(kindNodeImageTagEnv, tag)
			cluster, err := NewKindClusterFromEnvironment()
			if err == nil {
				t.Fatalf("NewKindClusterFromEnvironment() accepted invalid tag")
			}
			if cluster != nil {
				t.Fatalf("NewKindClusterFromEnvironment() cluster = %v on invalid tag, want nil", cluster)
			}
			if !strings.Contains(err.Error(), kindNodeImageTagEnv) {
				t.Fatalf("error %q does not identify %s", err, kindNodeImageTagEnv)
			}
		})
	}
}

func assertKindClusterImage(t *testing.T, cluster *kind.Cluster, want string) {
	t.Helper()
	capture := installFakeKind(t, cluster)

	_, err := cluster.Create(context.Background())
	if err == nil {
		t.Fatal("Create() error = nil, want fake kind create failure")
	}

	got, readErr := os.ReadFile(capture)
	if readErr != nil {
		t.Fatalf("failed to read fake kind arguments: %v", readErr)
	}
	args := strings.TrimSpace(string(got))
	base := "create cluster --name image-test"
	if want != "" {
		base += " --image " + want
	}
	if args != base {
		t.Fatalf("fake kind create args = %q, want %q", args, base)
	}
}

func assertKindCreateImage(t *testing.T, want string) {
	t.Helper()
	cluster, err := NewKindClusterFromEnvironment()
	if err != nil {
		t.Fatalf("NewKindClusterFromEnvironment() error = %v", err)
	}
	assertKindClusterImage(t, cluster, want)
}

func installFakeKind(t *testing.T, cluster *kind.Cluster) string {
	t.Helper()
	dir := t.TempDir()
	capture := filepath.Join(dir, "kind-args")
	binary := filepath.Join(dir, "kind")
	script := `#!/bin/sh
if [ "$1" = "get" ] && [ "$2" = "clusters" ]; then
  exit 0
fi
printf '%s\n' "$*" >> "$KIND_COMMAND_CAPTURE"
if [ "$1" = "create" ]; then
  exit 42
fi
exit 0
`
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to create fake kind executable: %v", err)
	}
	t.Setenv("KIND_COMMAND_CAPTURE", capture)
	cluster.WithName("image-test")
	cluster.WithOpts(kind.WithPath(binary))
	return capture
}

func unsetEnvironmentVariable(t *testing.T, name string) {
	t.Helper()
	original, wasSet := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("failed to unset %s: %v", name, err)
	}
	t.Cleanup(func() {
		var err error
		if wasSet {
			err = os.Setenv(name, original)
		} else {
			err = os.Unsetenv(name)
		}
		if err != nil {
			t.Errorf("failed to restore %s: %v", name, err)
		}
	})
}
