package e2e

import (
	"context"
	"flag"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Testing Velero on a kind cluster", func() {
	BeforeEach(func() {
		flag.Parse()
		ctx := context.TODO()
		err := EnsureClusterExists(ctx)
		Expect(err).To(Succeed())
	})
	Describe("Dummy test", func() {
		Context("Dummy test", func() {
			It("is a dummy test", func() {
			})
		})
	})
})
