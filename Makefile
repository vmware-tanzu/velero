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
PKG := github.com/heptio/velero

# Where to push the docker image.
REGISTRY ?= gcr.io/heptio-images

# Which architecture to build - see $(ALL_ARCH) for options.
# if the 'local' rule is being run, detect the ARCH from 'go env'
# if it wasn't specified by the caller.
local : ARCH ?= $(shell go env GOOS)-$(shell go env GOARCH)
ARCH ?= linux-amd64

VERSION ?= master

TAG_LATEST ?= false

###
### These variables should not need tweaking.
###

CLI_PLATFORMS := linux-amd64 linux-arm linux-arm64 darwin-amd64 windows-amd64
CONTAINER_PLATFORMS := linux-amd64 linux-arm linux-arm64

platform_temp = $(subst -, ,$(ARCH))
GOOS = $(word 1, $(platform_temp))
GOARCH = $(word 2, $(platform_temp))

# TODO(ncdc): support multiple image architectures once gcr.io supports manifest lists
# Set default base image dynamically for each arch
ifeq ($(GOARCH),amd64)
		DOCKERFILE ?= Dockerfile-$(BIN)
endif
#ifeq ($(GOARCH),arm)
#		DOCKERFILE ?= Dockerfile.arm #armel/busybox
#endif
#ifeq ($(GOARCH),arm64)
#		DOCKERFILE ?= Dockerfile.arm64 #aarch64/busybox
#endif

IMAGE = $(REGISTRY)/$(BIN)

# If you want to build all binaries, see the 'all-build' rule.
# If you want to build all containers, see the 'all-container' rule.
# If you want to build AND push all containers, see the 'all-push' rule.
all:
	@$(MAKE) build
	@$(MAKE) build BIN=velero-restic-restore-helper

build-%:
	@$(MAKE) --no-print-directory ARCH=$* build

#container-%:
#	@$(MAKE) --no-print-directory ARCH=$* container

#push-%:
#	@$(MAKE) --no-print-directory ARCH=$* push

all-build: $(addprefix build-, $(CLI_PLATFORMS))

#all-container: $(addprefix container-, $(CONTAINER_PLATFORMS))

#all-push: $(addprefix push-, $(CONTAINER_PLATFORMS))

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

# Use a slightly customized build/push targets since we don't have a Go binary to build for the fsfreeze image
build-fsfreeze: BIN = fsfreeze-pause
build-fsfreeze:
	@cp $(DOCKERFILE)  _output/.dockerfile-$(BIN).alpine
	@docker build -t $(IMAGE):$(VERSION) -f _output/.dockerfile-$(BIN).alpine _output
	@docker images -q $(IMAGE):$(VERSION) > .container-$(DOTFILE_IMAGE)

push-fsfreeze: BIN = fsfreeze-pause
push-fsfreeze:
	@docker push $(IMAGE):$(VERSION)
ifeq ($(TAG_LATEST), true)
	docker tag $(IMAGE):$(VERSION) $(IMAGE):latest
	docker push $(IMAGE):latest
endif
	@docker images -q $(REGISTRY)/fsfreeze-pause:$(VERSION) > .container-$(DOTFILE_IMAGE)

all-containers:
	$(MAKE) container
	$(MAKE) container BIN=velero-restic-restore-helper
	$(MAKE) build-fsfreeze

container: verify test .container-$(DOTFILE_IMAGE) container-name
.container-$(DOTFILE_IMAGE): _output/bin/$(GOOS)/$(GOARCH)/$(BIN) $(DOCKERFILE)
	@cp $(DOCKERFILE) _output/.dockerfile-$(BIN)-$(GOOS)-$(GOARCH)
	@docker build -t $(IMAGE):$(VERSION) -f _output/.dockerfile-$(BIN)-$(GOOS)-$(GOARCH) _output
	@docker images -q $(IMAGE):$(VERSION) > $@

container-name:
	@echo "container: $(IMAGE):$(VERSION)"

all-push:
	$(MAKE) push
	$(MAKE) push BIN=velero-restic-restore-helper
	$(MAKE) push-fsfreeze


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
	cd hack/build-image && docker build -t $(BUILDER_IMAGE) .

clean:
	rm -rf .container-* _output/.dockerfile-* .push-*
	rm -rf .go _output
	docker rmi $(BUILDER_IMAGE)

ci: all verify test

changelog:
	hack/changelog.sh

release:
	hack/goreleaser.sh
