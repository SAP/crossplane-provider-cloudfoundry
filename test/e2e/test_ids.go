//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
)

const (
	buildIDEnvKey      = "BUILD_ID"
	defaultTestBuildID = "0000"
	testOrgName        = "cf-ci-e2e"
)

func testBuildID() string {
	if buildID := os.Getenv(buildIDEnvKey); buildID != "" {
		return buildID
	}

	return defaultTestBuildID
}

func runScopedName(base string) string {
	return base + "-" + testBuildID()
}

// crsDir resolves a CRS subdirectory relative to TEST_CRS_PATH (set by
// the Makefile when rendering into a build directory). Falls back to ./crs/.
func crsDir(rel string) string {
	if base := os.Getenv("TEST_CRS_PATH"); base != "" {
		return filepath.Join(base, rel)
	}
	return filepath.Join("./crs", rel)
}
