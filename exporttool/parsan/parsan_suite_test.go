package parsan_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestParsan(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Parsan Suite")
}
