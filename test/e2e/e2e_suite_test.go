package e2e

import (
	"flag"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	veleroCLI            string
	veleroImage          string
	providerName         string // provider name for backup and volume storage: aws, vsphere, azure
	cloudCredentialsFile string
)

func init() {
	flag.StringVar(&veleroCLI, "velerocli", "velero", "path to the velero application to use")
	flag.StringVar(&providerName, "provider", "aws", "provider name for backup and volume storage (aws, vsphere, azure)")
	flag.StringVar(&veleroImage, "velero-image", "velero/velero:e2e-test", "image for the velero server to be tested")
	flag.StringVar(&cloudCredentialsFile, "credentials-file", "", "file containing credentials for backup and volume provider.")
}

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2e Suite")
}
