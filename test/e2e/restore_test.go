package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// Test a Velero backup and restore with APIGroupVersionsFeatureFlag enabled.
var _ = Describe("Test a Velero backup and restore with APIGroupVersionsFeatureFlag enabled", func() {
	Context("When backing up and restoring a namespace", func() {
		It("should be successfully backed up and restored", func() {
			Expect(RunRestoreWithAPIGroupVersionsFeatureFlag()).To(Succeed(), "Backing up rockbands namespace")
		})
	})
})
