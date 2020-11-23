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
	kibishiNamespace     string
	cloudPlatform        string // aws, vsphere, azure
	cloudCredentialsFile string
)

func init() {
	flag.StringVar(&veleroCLI, "velerocli", "velero", "path to the velero application to use")
	flag.StringVar(&kibishiNamespace, "kibishiins", "kibishii", "namespace to use for Kibishii distributed data generator")
	flag.StringVar(&cloudPlatform, "cloudplatform", "aws", "cloud platform we are deploying on (aws, vsphere, azure)")
	flag.StringVar(&veleroImage, "velero-image", "velero/velero:e2e-test", "image for the velero server to be tested")
	flag.StringVar(&cloudCredentialsFile, "credentials-file", "", "file containing credentials for backup and volume provider.")
}

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2e Suite")
}
