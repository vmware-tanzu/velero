# Copyright 2016 The Kubernetes Authors.
#
# Modifications Copyright 2017 the Velero contributors.
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

# The binary to build (just the basename).
BIN ?= velero

# This repo's root import path (under GOPATH).
PKG := github.com/vmware-tanzu/velero

# Where to push the docker image.
REGISTRY ?= velero

# Which architecture to build - see $(ALL_ARCH) for options.
# if the 'local' rule is being run, detect the ARCH from 'go env'
# if it wasn't specified by the caller.
local : ARCH ?= $(shell go env GOOS)-$(shell go env GOARCH)
ARCH ?= linux-amd64

VERSION ?= master

TAG_LATEST ?= false

# The version of restic binary to be downloaded for power architecture
RESTIC_VERSION ?= 0.9.5

CLI_PLATFORMS ?= linux-amd64 linux-arm linux-arm64 darwin-amd64 windows-amd64 linux-ppc64le
CONTAINER_PLATFORMS ?= linux-amd64 linux-ppc64le #linux-arm linux-arm64
MANIFEST_PLATFORMS ?= amd64 ppc64le

###
### These variables should not need tweaking.
###

platform_temp = $(subst -, ,$(ARCH))
GOOS = $(word 1, $(platform_temp))
GOARCH = $(word 2, $(platform_temp))

# Set default base image dynamically for each arch
ifeq ($(GOARCH),amd64)
		DOCKERFILE ?= Dockerfile-$(BIN)
local-arch:
	@echo "local environment for amd64 is up-to-date"
endif
#ifeq ($(GOARCH),arm)
#		DOCKERFILE ?= Dockerfile.arm #armel/busybox
#endif
#ifeq ($(GOARCH),arm64)
#		DOCKERFILE ?= Dockerfile.arm64 #aarch64/busybox
#endif
ifeq ($(GOARCH),ppc64le)
                DOCKERFILE ?= Dockerfile-$(BIN)-ppc64le
local-arch:
	RESTIC_VERSION=$(RESTIC_VERSION) \
        ./hack/get-restic-ppc64le.sh
endif

MULTIARCH_IMAGE = $(REGISTRY)/$(BIN)
IMAGE = $(REGISTRY)/$(BIN)-$(GOARCH)

# If you want to build all binaries, see the 'all-build' rule.
# If you want to build all containers, see the 'all-containers' rule.
# If you want to build AND push all containers, see the 'all-push' rule.
all:
	@$(MAKE) build
	@$(MAKE) build BIN=velero-restic-restore-helper

build-%:
	@$(MAKE) --no-print-directory ARCH=$* build
	@$(MAKE) --no-print-directory ARCH=$* build BIN=velero-restic-restore-helper

container-%:
	@$(MAKE) --no-print-directory ARCH=$* container
	@$(MAKE) --no-print-directory ARCH=$* container BIN=velero-restic-restore-helper

push-%:
	@$(MAKE) --no-print-directory ARCH=$* push
	@$(MAKE) --no-print-directory ARCH=$* push BIN=velero-restic-restore-helper

all-build: $(addprefix build-, $(CLI_PLATFORMS))

all-containers: $(addprefix container-, $(CONTAINER_PLATFORMS))

all-push: $(addprefix push-, $(CONTAINER_PLATFORMS))

all-manifests:
	@$(MAKE) manifest
	@$(MAKE) manifest BIN=velero-restic-restore-helper

local: build-dirs
	GOOS=$(GOOS) \
	GOARCH=$(GOARCH) \
	VERSION=$(VERSION) \
	PKG=$(PKG) \
	BIN=$(BIN) \
	OUTPUT_DIR=$$(pwd)/_output/bin/$(GOOS)/$(GOARCH) \
	./hack/build.sh

build: _output/bin/$(GOOS)/$(GOARCH)/$(BIN)

_output/bin/$(GOOS)/$(GOARCH)/$(BIN): build-dirs
	@echo "building: $@"
	$(MAKE) shell CMD="-c '\
		GOOS=$(GOOS) \
		GOARCH=$(GOARCH) \
		VERSION=$(VERSION) \
		PKG=$(PKG) \
		BIN=$(BIN) \
		OUTPUT_DIR=/output/$(GOOS)/$(GOARCH) \
		./hack/build.sh'"

TTY := $(shell tty -s && echo "-t")

BUILDER_IMAGE := velero-builder

# Example: make shell CMD="date > datefile"
shell: build-dirs build-image
	@# the volume bind-mount of $PWD/vendor/k8s.io/api is needed for code-gen to
	@# function correctly (ref. https://github.com/kubernetes/kubernetes/pull/64567)
	@docker run \
		-e GOFLAGS \
		-i $(TTY) \
		--rm \
		-u $$(id -u):$$(id -g) \
		-v "$$(pwd)/vendor/k8s.io/api:/go/src/k8s.io/api:delegated" \
		-v "$$(pwd)/.go/pkg:/go/pkg:delegated" \
		-v "$$(pwd)/.go/std:/go/std:delegated" \
		-v "$$(pwd):/go/src/$(PKG):delegated" \
		-v "$$(pwd)/_output/bin:/output:delegated" \
		-v "$$(pwd)/.go/std/$(GOOS)/$(GOARCH):/usr/local/go/pkg/$(GOOS)_$(GOARCH)_static:delegated" \
		-v "$$(pwd)/.go/go-build:/.cache/go-build:delegated" \
		-w /go/src/$(PKG) \
		$(BUILDER_IMAGE) \
		/bin/sh $(CMD)

DOTFILE_IMAGE = $(subst :,_,$(subst /,_,$(IMAGE))-$(VERSION))

all-containers:
	$(MAKE) container
	$(MAKE) container BIN=velero-restic-restore-helper

container: local-arch .container-$(DOTFILE_IMAGE) container-name
.container-$(DOTFILE_IMAGE): _output/bin/$(GOOS)/$(GOARCH)/$(BIN) $(DOCKERFILE)
	@cp $(DOCKERFILE) _output/.dockerfile-$(BIN)-$(GOOS)-$(GOARCH)
	@docker build --pull -t $(IMAGE):$(VERSION) -f _output/.dockerfile-$(BIN)-$(GOOS)-$(GOARCH) _output
	@docker images -q $(IMAGE):$(VERSION) > $@

container-name:
	@echo "container: $(IMAGE):$(VERSION)"

push: .push-$(DOTFILE_IMAGE) push-name
.push-$(DOTFILE_IMAGE): .container-$(DOTFILE_IMAGE)
	@docker push $(IMAGE):$(VERSION)
ifeq ($(TAG_LATEST), true)
	docker tag $(IMAGE):$(VERSION) $(IMAGE):latest
	docker push $(IMAGE):latest
endif
	@docker images -q $(IMAGE):$(VERSION) > $@

push-name:
	@echo "pushed: $(IMAGE):$(VERSION)"

manifest: .manifest-$(MULTIARCH_IMAGE) manifest-name
.manifest-$(MULTIARCH_IMAGE):
	@DOCKER_CLI_EXPERIMENTAL=enabled docker manifest create $(MULTIARCH_IMAGE):$(VERSION) \
		$(foreach arch, $(MANIFEST_PLATFORMS), $(MULTIARCH_IMAGE)-$(arch):$(VERSION))
	@DOCKER_CLI_EXPERIMENTAL=enabled docker manifest push --purge $(MULTIARCH_IMAGE):$(VERSION)
ifeq ($(TAG_LATEST), true)
	@DOCKER_CLI_EXPERIMENTAL=enabled docker manifest create $(MULTIARCH_IMAGE):latest \
		$(foreach arch, $(MANIFEST_PLATFORMS), $(MULTIARCH_IMAGE)-$(arch):latest)
	@DOCKER_CLI_EXPERIMENTAL=enabled docker manifest push --purge $(MULTIARCH_IMAGE):latest
endif

manifest-name:
	@echo "pushed: $(MULTIARCH_IMAGE):$(VERSION)"

SKIP_TESTS ?=
test: build-dirs
ifneq ($(SKIP_TESTS), 1)
	@$(MAKE) shell CMD="-c 'hack/test.sh $(WHAT)'"
endif

test-local: build-dirs
ifneq ($(SKIP_TESTS), 1)
	hack/test.sh $(WHAT)
endif

verify:
ifneq ($(SKIP_TESTS), 1)
	@$(MAKE) shell CMD="-c 'hack/verify-all.sh'"
endif

update:
	@$(MAKE) shell CMD="-c 'hack/update-all.sh'"

build-dirs:
	@mkdir -p _output/bin/$(GOOS)/$(GOARCH)
	@mkdir -p .go/src/$(PKG) .go/pkg .go/bin .go/std/$(GOOS)/$(GOARCH) .go/go-build

build-image:
	cd hack/build-image && docker build --pull -t $(BUILDER_IMAGE) .

clean:
	rm -rf .container-* _output/.dockerfile-* .push-*
	rm -rf .go _output
	docker rmi $(BUILDER_IMAGE)

ci: all verify test

changelog:
	hack/changelog.sh

# release builds a GitHub release using goreleaser within the build container.
#
# To dry-run the release, which will build the binaries/artifacts locally but
# will *not* create a GitHub release:
#		GITHUB_TOKEN=an-invalid-token-so-you-dont-accidentally-push-release \
#		RELEASE_NOTES_FILE=changelogs/CHANGELOG-1.2.md \
#		PUBLISH=false \
#		make release
#
# To run the release, which will publish a *DRAFT* GitHub release in github.com/vmware-tanzu/velero 
# (you still need to review/publish the GitHub release manually):
#		GITHUB_TOKEN=your-github-token \ 
#		RELEASE_NOTES_FILE=changelogs/CHANGELOG-1.2.md \
#		PUBLISH=true \
#		make release
release:
	$(MAKE) shell CMD="-c '\
		GITHUB_TOKEN=$(GITHUB_TOKEN) \
		RELEASE_NOTES_FILE=$(RELEASE_NOTES_FILE) \
		PUBLISH=$(PUBLISH) \
		./hack/goreleaser.sh'"

serve-docs:
	docker run \
	--rm \
	-v "$$(pwd)/site:/srv/jekyll" \
	-it -p 4000:4000 \
	jekyll/jekyll \
	jekyll serve --livereload --incremental

# gen-docs generates a new versioned docs directory under site/docs. It follows
# the following process:
#   1. Copies the contents of the most recently tagged docs directory into the new
#      directory, to establish a useful baseline to diff against.
#   2. Adds all copied content from step 1 to git's staging area via 'git add'.
#   3. Replaces the contents of the new docs directory with the contents of the
#      'master' docs directory, updating any version-specific links (e.g. to a
#      specific branch of the GitHub repository) to use the new version
#   4. Copies the previous version's ToC file and runs 'git add' to establish
#      a useful baseline to diff against.
#   5. Replaces the content of the new ToC file with the master ToC.
#   6. Update site/_config.yml and site/_data/toc-mapping.yml to include entries
#      for the new version.
#
# The unstaged changes in the working directory can now easily be diff'ed against the
# staged changes using 'git diff' to review all docs changes made since the previous 
# tagged version. Once the unstaged changes are ready, they can be added to the
# staging area using 'git add' and then committed.
#
# To run gen-docs: "NEW_DOCS_VERSION=v1.1.0 make gen-docs"
#
# **NOTE**: there are additional manual steps required to finalize the process of generating
# a new versioned docs site. The full process is documented in site/README-JEKYLL.md.
gen-docs:
	@hack/gen-docs.sh
