package e2etestify

import (
	"context"
	"flag"
	"os/exec"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"
	"github.com/vmware-tanzu/velero/test/e2e"
)

var (
	// TODO: Make variable test and other timeout flags/settings configurable.
	bslBucket            = flag.String("bucket", "", "name of the object storage bucket where backups from e2e tests should be stored. Required.")
	bslConfig            = flag.String("bsl-config", "", "configuration to use for the backup storage location. Format is key1=value1,key2=value2")
	bslPrefix            = flag.String("prefix", "", "prefix under which all Velero data should be stored within the bucket. Optional.")
	cloudCredentialsFile = flag.String("credentials-file", "", "file containing credentials for backup and volume provider. Required.")
	pluginProvider       = flag.String("plugin-provider", "", "Provider of object store and volume snapshotter plugins. Required.")
	veleroCLI            = flag.String("velerocli", "velero", "path to the velero application to use.")
	veleroImage          = flag.String("velero-image", "velero/velero:main", "image for the velero server to be tested.")
	vslConfig            = flag.String("vsl-config", "", "configuration to use for the volume snapshot location. Format is key1=value1,key2=value2")
)

type RestorePriorityAPIGVTests struct {
	*suite.Suite
	resource, group string
	certMgrCRD      map[string]string
	uuidgen         uuid.UUID
	client          *kubernetes.Clientset

	tests []struct {
		name       string
		namespaces []string
		srcCRD     map[string]string
		srcCRs     map[string]string
		tgtCRD     map[string]string
		tgtVer     string
		cm         *corev1api.ConfigMap
		gvs        map[string][]string
		want       map[string]map[string]string
	}
}

func (suite *RestorePriorityAPIGVTests) SetupSuite() {
	ctx := context.Background()

	suite.resource = "rockbands"
	suite.group = "music.example.io"
	suite.certMgrCRD = map[string]string{
		"url":       "https://github.com/jetstack/cert-manager/releases/download/v1.0.3/cert-manager.yaml",
		"namespace": "cert-manager",
	}

	var err error
	suite.client, err = e2e.GetClusterClient()
	suite.Require().NoError(err, "get cluster client")

	err = InstallCRD(ctx, suite.certMgrCRD["url"], suite.certMgrCRD["namespace"])
	suite.Require().NoError(err, "install cert-manager CRD")
}

func (suite *RestorePriorityAPIGVTests) SetupTest() {
	var err error

	suite.uuidgen, err = uuid.NewRandom()
	suite.Require().NoError(err, "seed random uuid generator")
}

func (suite *RestorePriorityAPIGVTests) TearDownTest() {
	ctx := context.Background()

	cmd := exec.CommandContext(ctx, "kubectl", "delete", "namespace", "music-system")
	_, _, _ = veleroexec.RunCommand(cmd)

	cmd = exec.CommandContext(ctx, "kubectl", "delete", "crd", "rockbands.music.example.io")
	_, _, _ = veleroexec.RunCommand(cmd)
}

func (suite *RestorePriorityAPIGVTests) TearDownSuite() {
	ctx := context.Background()
	_ = DeleteCRD(ctx, suite.certMgrCRD["url"], suite.certMgrCRD["namespace"])

	// Uninstall Velero.
	if suite.client != nil {
		_ = suite.client.CoreV1().Namespaces().Delete(
			context.Background(),
			"velero",
			metav1.DeleteOptions{},
		)
	}
}

func TestRestoreWithAPIGroupVersionsFeatureFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("Skip Testify e2e tests")
		return
	}

	suite.Run(t, &RestorePriorityAPIGVTests{
		Suite: new(suite.Suite),
	})
}
