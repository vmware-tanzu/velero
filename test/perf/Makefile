# Copyright 2020 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# If you update this file, please follow:
# https://suva.sh/posts/well-documented-makefiles/

# Use GOPROXY environment variable if set

.DEFAULT_GOAL:=help

ARCH ?= $(shell go env GOOS)-$(shell go env GOARCH)
platform_temp = $(subst -, ,$(ARCH))
GOOS = $(word 1, $(platform_temp))
GOARCH = $(word 2, $(platform_temp))

GOPROXY := $(shell go env GOPROXY)
ifeq ($(GOPROXY),)
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

REPO_ROOT := $(shell git rev-parse --show-toplevel)

help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z0-9_-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Binaries
## --------------------------------------

TOOLS_DIR := $(REPO_ROOT)/hack/tools
BIN_DIR := bin
TOOLS_BIN_DIR := $(TOOLS_DIR)/$(BIN_DIR)
GINKGO := $(GOPATH)/bin/ginkgo
KUSTOMIZE := $(TOOLS_BIN_DIR)/kustomize
OUTPUT_DIR := _output/$(GOOS)/$(GOARCH)/bin
GINKGO_FOCUS ?=
GINKGO_SKIP ?=
SKIP_STR := $(foreach var, $(subst ., ,$(GINKGO_SKIP)),-skip "$(var)")
FOCUS_STR := $(foreach var, $(subst ., ,$(GINKGO_FOCUS)),-focus "$(var)")
VELERO_CLI ?=$$(pwd)/../../_output/bin/$(GOOS)/$(GOARCH)/velero
VELERO_IMAGE ?= velero/velero:main
VELERO_VERSION ?= $(VERSION)
PLUGINS ?=
RESTORE_HELPER_IMAGE ?=

VELERO_NAMESPACE ?= velero
CREDS_FILE ?=
BSL_BUCKET ?=
BSL_PREFIX ?=
BSL_CONFIG ?=
VSL_CONFIG ?=
CLOUD_PROVIDER ?=
OBJECT_STORE_PROVIDER ?=
INSTALL_VELERO ?= true
REGISTRY_CREDENTIAL_FILE ?=

FEATURES ?=
DEBUG_E2E_TEST ?= false
VELERO_SERVER_DEBUG_MODE ?= false
NFS_SERVER_PATH ?=
# Parameters to run migration tests along with all other E2E tests, and both of them should
#   be provided or left them all empty to skip migration tests with no influence to other
#   E2E tests.
UPLOADER_TYPE ?=
TEST_CASE_DESCRIBE ?= 'velero performance test'
BACKUP_FOR_RESTORE ?=
Delete_Cluster_Resource ?= false
Debug_Velero_Pod_Restart ?= false
NODE_AGENT_POD_CPU_LIMIT ?= 4
NODE_AGENT_POD_MEM_LIMIT ?= 4Gi
NODE_AGENT_POD_CPU_REQUEST ?= 2
NODE_AGENT_POD_MEM_REQUEST ?= 2Gi
VELERO_POD_CPU_LIMIT ?= 4
VELERO_POD_MEM_LIMIT ?= 4Gi
VELERO_POD_CPU_REQUEST ?= 2
VELERO_POD_MEM_REQUEST ?= 2Gi
POD_VOLUME_OPERATION_TIMEOUT ?= 6h

.PHONY:ginkgo
ginkgo: # Make sure ginkgo is in $GOPATH/bin
	go install github.com/onsi/ginkgo/ginkgo@v1.16.5

.PHONY: run
run: ginkgo
		@[ "${CREDS_FILE}" ] && echo "Using credentials from ${CREDS_FILE}" || \
			( echo "A credentials file is required to run E2E tests, please re-run the make target with CREDS_FILE=<PathToCredentialsFile>"; exit 1 )
		@[ "${BSL_BUCKET}" ] && echo "Using bucket ${BSL_BUCKET} to store backups from E2E tests" || \
			(echo "Bucket to store the backups from E2E tests is required, please re-run with BSL_BUCKET=<BucketName>"; exit 1 )
		@[ "${CLOUD_PROVIDER}" ] && echo "Using cloud provider ${CLOUD_PROVIDER}" || \
			(echo "Cloud provider for target cloud/plugin provider is required, please rerun with CLOUD_PROVIDER=<aws,azure,kind,vsphere>"; exit 1)
	@$(GINKGO) -v $(FOCUS_STR) $(SKIP_STR) . -- -velerocli=$(VELERO_CLI) \
		-velero-image=$(VELERO_IMAGE) \
		-plugins=$(PLUGINS) \
		-velero-version=$(VELERO_VERSION) \
		-restore-helper-image=$(RESTORE_HELPER_IMAGE) \
		-velero-namespace=$(VELERO_NAMESPACE) \
		-credentials-file=$(CREDS_FILE) \
		-bucket=$(BSL_BUCKET) \
		-prefix=$(BSL_PREFIX) \
		-bsl-config=$(BSL_CONFIG) \
		-vsl-config=$(VSL_CONFIG) \
		-cloud-provider=$(CLOUD_PROVIDER) \
		-object-store-provider="$(OBJECT_STORE_PROVIDER)" \
		-features=$(FEATURES) \
		-install-velero=$(INSTALL_VELERO) \
		-registry-credential-file=$(REGISTRY_CREDENTIAL_FILE) \
		-debug-e2e-test=$(DEBUG_E2E_TEST) \
		-velero-server-debug-mode=$(VELERO_SERVER_DEBUG_MODE) \
		-uploader-type=$(UPLOADER_TYPE) \
		-nfs-server-path=$(NFS_SERVER_PATH) \
		-test-case-describe=$(TEST_CASE_DESCRIBE) \
		-backup-for-restore=$(BACKUP_FOR_RESTORE) \
		-delete-cluster-resource=$(Delete_Cluster_Resource) \
		-debug-velero-pod-restart=$(Debug_Velero_Pod_Restart) \
		-node-agent-pod-cpu-limit=$(NODE_AGENT_POD_CPU_LIMIT) \
		-node-agent-pod-mem-limit=$(NODE_AGENT_POD_MEM_LIMIT) \
		-node-agent-pod-cpu-request=$(NODE_AGENT_POD_CPU_REQUEST) \
		-node-agent-pod-mem-request=$(NODE_AGENT_POD_MEM_REQUEST) \
		-velero-pod-cpu-limit=$(VELERO_POD_CPU_LIMIT) \
		-velero-pod-mem-limit=$(VELERO_POD_MEM_LIMIT) \
		-velero-pod-cpu-request=$(VELERO_POD_CPU_REQUEST) \
		-velero-pod-mem-request=$(VELERO_POD_MEM_REQUEST) \
		-pod-volume-operation-timeout=$(POD_VOLUME_OPERATION_TIMEOUT)

build: ginkgo
	mkdir -p $(OUTPUT_DIR)
	$(GINKGO) build . 
