package e2e

import (
	"context"
	"flag"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Testing Velero on an aws cluster", func() {
	BeforeEach(func() {
		flag.Parse()
		ctx := context.TODO()
		err := EnsureClusterExists(ctx)
		Expect(err).NotTo(HaveOccurred())
	})
	Describe("", func() {
		Context("Dummy test", func() {
			It("is a dummy test", func() {
			})
		})
	})
})
