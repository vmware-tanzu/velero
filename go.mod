module github.com/vmware-tanzu/velero

go 1.13

require (
	cloud.google.com/go v0.46.2 // indirect
	github.com/Azure/azure-sdk-for-go v21.4.0+incompatible
	github.com/Azure/go-autorest v11.1.2+incompatible
	github.com/aws/aws-sdk-go v1.16.26
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/docker/spdystream v0.0.0-20170912183627-bc6354cbbc29 // indirect
	github.com/evanphx/json-patch v0.0.0-20190203023257-5858425f7550
	github.com/gobwas/glob v0.2.3
	github.com/gofrs/uuid v3.2.0+incompatible
	github.com/golang/protobuf v1.3.2
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/googleapis/gnostic v0.1.0 // indirect
	github.com/hashicorp/go-hclog v0.0.0-20180709165350-ff2cf002a8dd
	github.com/hashicorp/go-plugin v1.0.1-0.20190610192547-a1bc61569a26
	github.com/joho/godotenv v1.3.0
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.2.0
	github.com/robfig/cron v1.0.1-0.20170309132418-df38d32658d8
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/afero v0.0.0-20170217164146-9be650865eab
	github.com/spf13/cobra v0.0.3
	github.com/spf13/pflag v1.0.2
	github.com/stretchr/testify v1.3.0
	golang.org/x/net v0.0.0-20190812203447-cdfb69ac37fc
	google.golang.org/grpc v1.21.1
	k8s.io/api v0.0.0
	k8s.io/apiextensions-apiserver v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/client-go v0.0.0
	k8s.io/klog v0.3.1
	k8s.io/kubernetes v1.15.3
	k8s.io/utils v0.0.0-20190308190857-21c4ce38f2a7 // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.0.0-20190620084959-7cf5895f2711
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190620085554-14e95df34f1f
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190612205821-1799e75a0719
	k8s.io/apiserver => k8s.io/apiserver v0.0.0-20190620085212-47dc9a115b18
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20190620085706-2090e6d8f84c
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20190620090043-8301c0bda1f0
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20190620090013-c9a0fc045dc1
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20190612205613-18da4a14b22b
	k8s.io/component-base => k8s.io/component-base v0.0.0-20190620085130-185d68e6e6ea
	k8s.io/cri-api => k8s.io/cri-api v0.0.0-20190531030430-6117653b35f1
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.0.0-20190620090116-299a7b270edc
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20190620085325-f29e2b4a4f84
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.0.0-20190620085942-b7f18460b210
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.0.0-20190620085809-589f994ddf7f
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.0.0-20190620085912-4acac5405ec6
	k8s.io/kubelet => k8s.io/kubelet v0.0.0-20190620085838-f1cb295a73c9
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20190620090156-2138f2c9de18
	k8s.io/metrics => k8s.io/metrics v0.0.0-20190620085625-3b22d835f165
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.0.0-20190620085408-1aef9010884e
)
