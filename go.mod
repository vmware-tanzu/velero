module github.com/vmware-tanzu/velero

go 1.16

require (
	github.com/Azure/azure-sdk-for-go v42.0.0+incompatible
	github.com/Azure/go-autorest/autorest v0.11.18
	github.com/Azure/go-autorest/autorest/azure/auth v0.4.2
	github.com/Azure/go-autorest/autorest/to v0.3.0
	github.com/Azure/go-autorest/autorest/validation v0.2.0 // indirect
	github.com/aws/aws-sdk-go v1.28.2
	github.com/evanphx/json-patch v4.11.0+incompatible
	github.com/fatih/color v1.10.0
	github.com/gobwas/glob v0.2.3
	github.com/gofrs/uuid v3.2.0+incompatible
	github.com/golang/protobuf v1.5.2
	github.com/google/uuid v1.1.2
	github.com/hashicorp/go-hclog v0.0.0-20180709165350-ff2cf002a8dd
	github.com/hashicorp/go-plugin v0.0.0-20190610192547-a1bc61569a26
	github.com/joho/godotenv v1.3.0
	github.com/kubernetes-csi/external-snapshotter/client/v4 v4.0.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/robfig/cron v1.1.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/afero v1.6.0
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/vmware-tanzu/crash-diagnostics v0.3.7
	golang.org/x/mod v0.4.2
	golang.org/x/net v0.0.0-20210520170846-37e1c6afe023
	google.golang.org/grpc v1.38.0
	k8s.io/api v0.22.2
	k8s.io/apiextensions-apiserver v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/cli-runtime v0.22.2
	k8s.io/client-go v0.22.2
	k8s.io/klog v1.0.0
	k8s.io/kube-aggregator v0.19.12
	sigs.k8s.io/cluster-api v0.3.11-0.20210106212952-b6c1b5b3db3d
	sigs.k8s.io/controller-runtime v0.10.2
)

replace github.com/gogo/protobuf => github.com/gogo/protobuf v1.3.2
