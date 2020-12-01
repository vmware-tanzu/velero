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
	cloudCredentialsFile string
	bslConfig            string
	bslBucket            string
	bslPrefix            string
	vslConfig            string
)

func init() {
	flag.StringVar(&veleroCLI, "velerocli", "velero", "path to the velero application to use")
	flag.StringVar(&veleroImage, "velero-image", "velero/velero:e2e-test", "image for the velero server to be tested")
	flag.StringVar(&cloudCredentialsFile, "credentials-file", "", "file containing credentials for backup and volume provider.")
	flag.StringVar(&bslConfig, "bsl-config", "", "configuration to use for the backup storage location. Format is key1=value1,key2=value2")
	flag.StringVar(&bslBucket, "bucket", "", "name of the object storage bucket where backups from e2e tests should be stored")
	flag.StringVar(&bslPrefix, "prefix", "", "prefix under which all Velero data should be stored within the bucket. Optional.")
	flag.StringVar(&vslConfig, "vsl-config", "", "configuration to use for the volume snapshot location. Format is key1=value1,key2=value2")
}

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2e Suite")
}
