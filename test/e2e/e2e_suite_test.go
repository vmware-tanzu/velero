package e2e

import (
	"flag"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	veleroCLI        string
	kibishiNamespace string
	cloudPlatform    string // aws, vsphere, azure
)

func init() {
	flag.StringVar(&veleroCLI, "velerocli", "velero", "path to the velero application to use")
	flag.StringVar(&kibishiNamespace, "kibishiins", "kibishii", "namespace to use for Kibishii distributed data generator")
	flag.StringVar(&cloudPlatform, "cloudplatform", "aws", "cloud platform we are deploying on (aws, vsphere, azure)")
}

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2e Suite")
}
