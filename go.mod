module github.com/vmware-tanzu/velero

go 1.22.0

require (
	cloud.google.com/go/storage v1.40.0
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.11.1
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.6.0
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5 v5.6.0
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage v1.5.0
	github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v1.3.2
	github.com/aws/aws-sdk-go-v2 v1.24.1
	github.com/aws/aws-sdk-go-v2/config v1.26.3
	github.com/aws/aws-sdk-go-v2/credentials v1.16.14
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.15.11
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.143.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.48.0
	github.com/aws/aws-sdk-go-v2/service/sts v1.26.7
	github.com/bombsimon/logrusr/v3 v3.0.0
	github.com/evanphx/json-patch/v5 v5.8.0
	github.com/fatih/color v1.16.0
	github.com/gobwas/glob v0.2.3
	github.com/google/go-cmp v0.6.0
	github.com/google/uuid v1.6.0
	github.com/hashicorp/go-hclog v0.14.1
	github.com/hashicorp/go-plugin v1.6.0
	github.com/joho/godotenv v1.3.0
	github.com/kopia/kopia v0.16.0
	github.com/kubernetes-csi/external-snapshotter/client/v7 v7.0.0
	github.com/onsi/ginkgo/v2 v2.19.0
	github.com/onsi/gomega v1.33.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.19.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/afero v1.6.0
	github.com/spf13/cobra v1.7.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.9.0
	github.com/vmware-tanzu/crash-diagnostics v0.3.7
	go.uber.org/zap v1.27.0
	golang.org/x/exp v0.0.0-20230522175609-2e198f4a06a1
	golang.org/x/mod v0.17.0
	golang.org/x/net v0.26.0
	golang.org/x/oauth2 v0.19.0
	golang.org/x/text v0.16.0
	google.golang.org/api v0.172.0
	google.golang.org/grpc v1.63.2
	google.golang.org/protobuf v1.33.0
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.29.0
	k8s.io/apiextensions-apiserver v0.29.0
	k8s.io/apimachinery v0.29.0
	k8s.io/cli-runtime v0.24.0
	k8s.io/client-go v0.29.0
	k8s.io/klog/v2 v2.110.1
	k8s.io/kube-aggregator v0.19.12
	k8s.io/metrics v0.25.6
	k8s.io/utils v0.0.0-20230726121419-3b25d923346b
	sigs.k8s.io/controller-runtime v0.17.2
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd
	sigs.k8s.io/yaml v1.4.0
)

require (
	cloud.google.com/go v0.112.1 // indirect
	cloud.google.com/go/compute v1.24.0 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	cloud.google.com/go/iam v1.1.7 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.8.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.2.2 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.5.4 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.14.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.2.10 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.5.10 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.7.2 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.2.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.10.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.2.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.10.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.16.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.18.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.21.6 // indirect
	github.com/aws/smithy-go v1.19.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/chmduquesne/rollinghash v4.0.0+incompatible // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/edsrzf/mmap-go v1.1.0 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/gofrs/flock v0.8.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.1 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20240424215950-a892ee059fd6 // indirect
	github.com/google/s2a-go v0.1.7 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/googleapis/gax-go/v2 v2.12.3 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/hashicorp/cronexpr v1.1.2 // indirect
	github.com/hashicorp/yamux v0.1.1 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.8 // indirect
	github.com/klauspost/cpuid/v2 v2.2.6 // indirect
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/klauspost/reedsolomon v1.12.1 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/minio/minio-go/v7 v7.0.69 // indirect
	github.com/minio/sha256-simd v1.0.1 // indirect
	github.com/mitchellh/go-testing-interface v1.0.0 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	github.com/mxk/go-vss v1.2.0 // indirect
	github.com/natefinch/atomic v1.0.1 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/oklog/run v1.0.0 // indirect
	github.com/petar/GoLLRB v0.0.0-20210522233825-ae3b015fd3e9 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.52.3 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/rs/xid v1.5.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/vladimirvivien/gexe v0.1.1 // indirect
	github.com/zeebo/blake3 v0.2.3 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.49.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.49.0 // indirect
	go.opentelemetry.io/otel v1.25.0 // indirect
	go.opentelemetry.io/otel/metric v1.25.0 // indirect
	go.opentelemetry.io/otel/trace v1.25.0 // indirect
	go.starlark.net v0.0.0-20201006213952-227f4aabceb5 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.24.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.21.0 // indirect
	golang.org/x/term v0.21.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/genproto v0.0.0-20240227224415-6ceb2ff114de // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240314234333-6e1732d8331c // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240401170217-c3f982113cda // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/component-base v0.29.0 // indirect
	k8s.io/kube-openapi v0.0.0-20231010175941-2dd684a91f00 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
)

replace github.com/kopia/kopia => github.com/project-velero/kopia v0.0.0-20241016073907-939dae5f9001
