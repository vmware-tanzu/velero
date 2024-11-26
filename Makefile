# Copyright 2016 The Kubernetes Authors.
#
# Modifications Copyright 2020 the Velero contributors.
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
GCR_REGISTRY ?= gcr.io/velero-gcp

# Image name
IMAGE ?= $(REGISTRY)/$(BIN)
GCR_IMAGE ?= $(GCR_REGISTRY)/$(BIN)

# We allow the Dockerfile to be configurable to enable the use of custom Dockerfiles
# that pull base images from different registries.
VELERO_DOCKERFILE ?= Dockerfile
BUILDER_IMAGE_DOCKERFILE ?= hack/build-image/Dockerfile

# Calculate the realpath of the build-image Dockerfile as we `cd` into the hack/build
# directory before this Dockerfile is used and any relative path will not be valid.
BUILDER_IMAGE_DOCKERFILE_REALPATH := $(shell realpath $(BUILDER_IMAGE_DOCKERFILE))

# Build image handling. We push a build image for every changed version of
# /hack/build-image/Dockerfile. We tag the dockerfile with the short commit hash
# of the commit that changed it. When determining if there is a build image in
# the registry to use we look for one that matches the current "commit" for the
# Dockerfile else we make one.
# In the case where the Dockerfile for the build image has been overridden using
# the BUILDER_IMAGE_DOCKERFILE variable, we always force a build.

ifneq "$(origin BUILDER_IMAGE_DOCKERFILE)" "file"
	BUILDER_IMAGE_TAG := "custom"
else
	BUILDER_IMAGE_TAG := $(shell git log -1 --pretty=%h $(BUILDER_IMAGE_DOCKERFILE))
endif

BUILDER_IMAGE := $(REGISTRY)/build-image:$(BUILDER_IMAGE_TAG)
BUILDER_IMAGE_CACHED := $(shell docker images -q ${BUILDER_IMAGE} 2>/dev/null )

HUGO_IMAGE := hugo-builder

# Which architecture to build - see $(ALL_ARCH) for options.
# if the 'local' rule is being run, detect the ARCH from 'go env'
# if it wasn't specified by the caller.
local : ARCH ?= $(shell go env GOOS)-$(shell go env GOARCH)
ARCH ?= linux-amd64

VERSION ?= main

TAG_LATEST ?= false

ifeq ($(TAG_LATEST), true)
	IMAGE_TAGS ?= $(IMAGE):$(VERSION) $(IMAGE):latest
	GCR_IMAGE_TAGS ?= $(GCR_IMAGE):$(VERSION) $(GCR_IMAGE):latest
else
	IMAGE_TAGS ?= $(IMAGE):$(VERSION)
	GCR_IMAGE_TAGS ?= $(GCR_IMAGE):$(VERSION)
endif

# check buildx is enabled only if docker is in path
# macOS/Windows docker cli without Docker Desktop license: https://github.com/abiosoft/colima
# To add buildx to docker cli: https://github.com/abiosoft/colima/discussions/273#discussioncomment-2684502
ifeq ($(shell which docker 2>/dev/null 1>&2 && docker buildx inspect 2>/dev/null | awk '/Status/ { print $$2 }'), running)
	BUILDX_ENABLED ?= true
# if emulated docker cli from podman, assume enabled
# emulated docker cli from podman: https://podman-desktop.io/docs/migrating-from-docker/emulating-docker-cli-with-podman
# podman known issues:
# - on remote podman, such as on macOS,
#   --output issue: https://github.com/containers/podman/issues/15922
else ifeq ($(shell which docker 2>/dev/null 1>&2 && cat $(shell which docker) | grep -c "exec podman"), 1)
	BUILDX_ENABLED ?= true
else
	BUILDX_ENABLED ?= false
endif

define BUILDX_ERROR
buildx not enabled, refusing to run this recipe
see: https://velero.io/docs/main/build-from-source/#making-images-and-updating-velero for more info
endef
# comma cannot be escaped and can only be used in Make function arguments by putting into variable
comma=,
# The version of restic binary to be downloaded
RESTIC_VERSION ?= 0.15.0

CLI_PLATFORMS ?= linux-amd64 linux-arm linux-arm64 darwin-amd64 darwin-arm64 windows-amd64 linux-ppc64le
BUILDX_PLATFORMS ?= $(subst -,/,$(ARCH))
# Whether or not buildx should push the image to the registry, applies to when BUILDX_PLATFORMS has multiple platforms below.
# false by default because most people do not have credentials to $(REGISTRY) nor should they push from their local development machine.
# once you have set $(REGISTRY) or $(IMAGE) and have credentials to those registries, you can set BUILDX_PUSH=true
BUILDX_PUSH ?= false
# if BUILDX_PLATFORMS has multiple platforms, we need to use BUILDX_OUTPUT_TYPE=image and optionally BUILDX_OUTPUT_TYPE=image,push=true if BUILDX_PUSH is true
# The default image store in Docker Engine doesn't support loading multi-platform images. So set BUILDX_PUSH=true to push the image to the registry if you need to use the
# multi-platform output image. In the future, we may add containerd support for multi-platform images. https://docs.docker.com/engine/storage/containerd/
# if BUILDX_PLATFORMS has only one platform, we can use BUILDX_OUTPUT_TYPE=docker to import the image to the local docker daemon
ifeq ($(words $(subst $(comma), ,$(BUILDX_PLATFORMS))),1)
	BUILDX_OUTPUT_TYPE ?= docker
else
	BUILDX_OUTPUT_TYPE ?= image$(subst false,,$(subst true,$(comma)push=true,$(BUILDX_PUSH)))
endif
# set git sha and tree state
GIT_SHA = $(shell git rev-parse HEAD)
ifneq ($(shell git status --porcelain 2> /dev/null),)
	GIT_TREE_STATE ?= dirty
else
	GIT_TREE_STATE ?= clean
endif

###
### These variables should not need tweaking.
###

platform_temp = $(subst -, ,$(ARCH))
GOOS = $(word 1, $(platform_temp))
GOARCH = $(word 2, $(platform_temp))
GOPROXY ?= https://proxy.golang.org
GOBIN=$$(pwd)/.go/bin

# If you want to build all binaries, see the 'all-build' rule.
# If you want to build all containers, see the 'all-containers' rule.
all:
	@$(MAKE) build
	@$(MAKE) build BIN=velero-restore-helper

build-%:
	@$(MAKE) --no-print-directory ARCH=$* build
	@$(MAKE) --no-print-directory ARCH=$* build BIN=velero-restore-helper

all-build: $(addprefix build-, $(CLI_PLATFORMS))

all-containers:
	@$(MAKE) --no-print-directory container
	@$(MAKE) --no-print-directory container BIN=velero-restore-helper

local: build-dirs
# Add DEBUG=1 to enable debug locally
	GOOS=$(GOOS) \
	GOARCH=$(GOARCH) \
	GOBIN=$(GOBIN) \
	VERSION=$(VERSION) \
	REGISTRY=$(REGISTRY) \
	PKG=$(PKG) \
	BIN=$(BIN) \
	GIT_SHA=$(GIT_SHA) \
	GIT_TREE_STATE=$(GIT_TREE_STATE) \
	OUTPUT_DIR=$$(pwd)/_output/bin/$(GOOS)/$(GOARCH) \
	./hack/build.sh

build: _output/bin/$(GOOS)/$(GOARCH)/$(BIN)

_output/bin/$(GOOS)/$(GOARCH)/$(BIN): build-dirs
	@echo "building: $@"
	$(MAKE) shell CMD="-c '\
		GOOS=$(GOOS) \
		GOARCH=$(GOARCH) \
		GOBIN=$(GOBIN) \
		VERSION=$(VERSION) \
		REGISTRY=$(REGISTRY) \
		PKG=$(PKG) \
		BIN=$(BIN) \
		GIT_SHA=$(GIT_SHA) \
		GIT_TREE_STATE=$(GIT_TREE_STATE) \
		OUTPUT_DIR=/output/$(GOOS)/$(GOARCH) \
		./hack/build.sh'"

TTY := $(shell tty -s && echo "-t")

# Example: make shell CMD="date > datefile"
shell: build-dirs build-env
	@# bind-mount the Velero root dir in at /github.com/vmware-tanzu/velero
	@# because the Kubernetes code-generator tools require the project to
	@# exist in a directory hierarchy ending like this (but *NOT* necessarily
	@# under $GOPATH).
	@docker run \
		-e GOFLAGS \
		-e GOPROXY \
		-i $(TTY) \
		--rm \
		-u $$(id -u):$$(id -g) \
		-v "$$(pwd):/github.com/vmware-tanzu/velero:delegated" \
		-v "$$(pwd)/_output/bin:/output:delegated" \
		-v "$$(pwd)/.go/pkg:/go/pkg:delegated" \
		-v "$$(pwd)/.go/std:/go/std:delegated" \
		-v "$$(pwd)/.go/std/$(GOOS)/$(GOARCH):/usr/local/go/pkg/$(GOOS)_$(GOARCH)_static:delegated" \
		-v "$$(pwd)/.go/go-build:/.cache/go-build:delegated" \
		-v "$$(pwd)/.go/golangci-lint:/.cache/golangci-lint:delegated" \
		-w /github.com/vmware-tanzu/velero \
		$(BUILDER_IMAGE) \
		/bin/sh $(CMD)

container:
ifneq ($(BUILDX_ENABLED), true)
	$(error $(BUILDX_ERROR))
endif
	@docker buildx build --pull \
	--output=type=$(BUILDX_OUTPUT_TYPE) \
	--platform $(BUILDX_PLATFORMS) \
	$(addprefix -t , $(IMAGE_TAGS)) \
	$(addprefix -t , $(GCR_IMAGE_TAGS)) \
	--build-arg=GOPROXY=$(GOPROXY) \
	--build-arg=PKG=$(PKG) \
	--build-arg=BIN=$(BIN) \
	--build-arg=VERSION=$(VERSION) \
	--build-arg=GIT_SHA=$(GIT_SHA) \
	--build-arg=GIT_TREE_STATE=$(GIT_TREE_STATE) \
	--build-arg=REGISTRY=$(REGISTRY) \
	--build-arg=RESTIC_VERSION=$(RESTIC_VERSION) \
	-f $(VELERO_DOCKERFILE) .
	@echo "container: $(IMAGE):$(VERSION)"

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

lint:
ifneq ($(SKIP_TESTS), 1)
	@$(MAKE) shell CMD="-c 'hack/lint.sh'"
endif

local-lint:
ifneq ($(SKIP_TESTS), 1)
	@hack/lint.sh
endif

update:
	@$(MAKE) shell CMD="-c 'hack/update-all.sh'"

# update-crd is for development purpose only, it is faster than update, so is a shortcut when you want to generate CRD changes only
update-crd:
	@$(MAKE) shell CMD="-c 'hack/update-3generated-crd-code.sh'"	

build-dirs:
	@mkdir -p _output/bin/$(GOOS)/$(GOARCH)
	@mkdir -p .go/src/$(PKG) .go/pkg .go/bin .go/std/$(GOOS)/$(GOARCH) .go/go-build .go/golangci-lint

build-env:
	@# if we have overridden the value for the build-image Dockerfile,
	@# force a build using that Dockerfile
	@# if we detect changes in dockerfile force a new build-image
	@# else if we dont have a cached image make one
	@# finally use the cached image
ifneq "$(origin BUILDER_IMAGE_DOCKERFILE)" "file"
	@echo "Dockerfile for builder image has been overridden to $(BUILDER_IMAGE_DOCKERFILE)"
	@echo "Preparing a new builder-image"
	$(MAKE) build-image
else ifneq ($(shell git diff --quiet HEAD -- $(BUILDER_IMAGE_DOCKERFILE); echo $$?), 0)
	@echo "Local changes detected in $(BUILDER_IMAGE_DOCKERFILE)"
	@echo "Preparing a new builder-image"
	$(MAKE) build-image
else ifneq ($(BUILDER_IMAGE_CACHED),)
	@echo "Using Cached Image: $(BUILDER_IMAGE)"
else
	@echo "Trying to pull build-image: $(BUILDER_IMAGE)"
	docker pull -q $(BUILDER_IMAGE) || $(MAKE) build-image
endif

build-image:
	@# When we build a new image we just untag the old one.
	@# This makes sure we don't leave the orphaned image behind.
	$(eval old_id=$(shell docker image inspect  --format '{{ .ID }}' ${BUILDER_IMAGE} 2>/dev/null))
ifeq ($(BUILDX_ENABLED), true)
	@cd hack/build-image && docker buildx build --build-arg=GOPROXY=$(GOPROXY) --output=type=docker --pull -t $(BUILDER_IMAGE) -f $(BUILDER_IMAGE_DOCKERFILE_REALPATH) .
else
	@cd hack/build-image && docker build --build-arg=GOPROXY=$(GOPROXY) --pull -t $(BUILDER_IMAGE) -f $(BUILDER_IMAGE_DOCKERFILE_REALPATH) .
endif
	$(eval new_id=$(shell docker image inspect  --format '{{ .ID }}' ${BUILDER_IMAGE} 2>/dev/null))
	@if [ "$(old_id)" != "" ] && [ "$(old_id)" != "$(new_id)" ]; then \
		docker rmi -f $$id || true; \
	fi

push-build-image:
	@# this target will push the build-image it assumes you already have docker
	@# credentials needed to accomplish this.
	@# Pushing will be skipped if a custom Dockerfile was used to build the image.
ifneq "$(origin BUILDER_IMAGE_DOCKERFILE)" "file"
	@echo "Dockerfile for builder image has been overridden"
	@echo "Skipping push of custom image"
else
	docker push $(BUILDER_IMAGE)
endif

build-image-hugo:
	cd site && docker build --pull -t $(HUGO_IMAGE) .

clean:
# if we have a cached image then use it to run go clean --modcache
# this test checks if we there is an image id in the BUILDER_IMAGE_CACHED variable.
ifneq ($(strip $(BUILDER_IMAGE_CACHED)),)
	$(MAKE) shell CMD="-c 'go clean --modcache'"
	docker rmi -f $(BUILDER_IMAGE) || true
endif
	rm -rf .go _output
	docker rmi $(HUGO_IMAGE)


.PHONY: modules
modules:
	go mod tidy


.PHONY: verify-modules
verify-modules: modules
	@if !(git diff --quiet HEAD -- go.sum go.mod); then \
		echo "go module files are out of date, please commit the changes to go.mod and go.sum"; exit 1; \
	fi


ci: verify-modules verify all test


changelog:
	hack/release-tools/changelog.sh

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
		REGISTRY=$(REGISTRY) \
		./hack/release-tools/goreleaser.sh'"

serve-docs: build-image-hugo
	docker run \
	--rm \
	-v "$$(pwd)/site:/srv/hugo" \
	-it -p 1313:1313 \
	$(HUGO_IMAGE) \
	server --bind=0.0.0.0 --enableGitInfo=false
# gen-docs generates a new versioned docs directory under site/content/docs.
# Please read the documentation in the script for instructions on how to use it.
gen-docs:
	@hack/release-tools/gen-docs.sh

.PHONY: test-e2e
test-e2e: local
	$(MAKE) -e VERSION=$(VERSION) -C test/ run-e2e

.PHONY: test-perf
test-perf: local
	$(MAKE) -e VERSION=$(VERSION) -C test/ run-perf

go-generate:
	go generate ./pkg/...

# requires an authenticated gh cli
# gh: https://cli.github.com/
# First create a PR
# gh pr create --title 'Title name' --body 'PR body'
# by default uses PR title as changelog body but can be overwritten like so
# make new-changelog CHANGELOG_BODY="Changes you have made"
new-changelog: GH_LOGIN ?= $(shell gh pr view --json author --jq .author.login 2> /dev/null)
new-changelog: GH_PR_NUMBER ?= $(shell gh pr view --json number --jq .number 2> /dev/null)
new-changelog: CHANGELOG_BODY ?= '$(shell gh pr view --json title --jq .title)'
new-changelog:
	@if [ "$(GH_LOGIN)" = "" ]; then \
		echo "branch does not have PR or cli not logged in, try 'gh auth login' or 'gh pr create'"; \
		exit 1; \
	fi
	@mkdir -p ./changelogs/unreleased/ && \
	echo $(CHANGELOG_BODY) > ./changelogs/unreleased/$(GH_PR_NUMBER)-$(GH_LOGIN) && \
	echo \"$(CHANGELOG_BODY)\" added to "./changelogs/unreleased/$(GH_PR_NUMBER)-$(GH_LOGIN)"
