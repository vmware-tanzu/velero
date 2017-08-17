# Copyright 2017 Heptio Inc.
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

# project related vars
ROOT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
PROJECT = ark
VERSION ?= v0.3.3
OUTPUT_DIR = $(ROOT_DIR)/_output
BIN_DIR = $(OUTPUT_DIR)/bin

# The binary to build (just the basename).
BIN := ark

# Which architecture to build - see $(ALL_ARCH) for options.
ARCH ?= amd64
ALL_ARCH := amd64 arm arm64

# docker related vars
DOCKER ?= docker
REGISTRY ?= gcr.io/heptio-images
IMAGE := $(REGISTRY)/$(BIN)-$(ARCH)
BUILD_IMAGE ?= gcr.io/heptio-images/golang:1.8-alpine3.6

# This repo's root import path (under GOPATH).
PKG := github.com/heptio/ark

# Set default base image dynamically for each arch
ifeq ($(ARCH),amd64)
    BASEIMAGE?=alpine:3.6
endif
ifeq ($(ARCH),arm)
    BASEIMAGE?=multiarch/alpine:armhf-v3.6
endif
ifeq ($(ARCH),arm64)
    BASEIMAGE?=multiarch/alpine:aarch64-v3.6
endif

# test related vars
TESTARGS ?= -timeout 60s
TEST_PKGS ?= ./cmd/... ./pkg/...
SKIP_TESTS ?=

verify:
ifneq ($(SKIP_TESTS), 1)
	@echo "verifying:"
	${ROOT_DIR}/hack/verify-generated-docs.sh
	${ROOT_DIR}/hack/verify-generated-clientsets.sh
	${ROOT_DIR}/hack/verify-generated-listers.sh
	${ROOT_DIR}/hack/verify-generated-informers.sh
endif

update:
	@echo "updating:"
	${ROOT_DIR}/hack/update-generated-docs.sh
	${ROOT_DIR}/hack/update-generated-clientsets.sh
	${ROOT_DIR}/hack/update-generated-listers.sh
	${ROOT_DIR}/hack/update-generated-informers.sh

# If you want to build all binaries, see the 'all-build' rule.
# If you want to build all containers, see the 'all-container' rule.
# If you want to build AND push all containers, see the 'all-push' rule.
all: test build

build-%: verify update
	@$(MAKE) --no-print-directory ARCH=$* build

container-%:
	@$(MAKE) --no-print-directory ARCH=$* container

push-%:
	@$(MAKE) --no-print-directory ARCH=$* push

all-build: test $(addprefix build-, $(ALL_ARCH))

all-container: $(addprefix container-, $(ALL_ARCH))

all-push: $(addprefix push-, $(ALL_ARCH))

build: bin/$(ARCH)/$(BIN)

bin/$(ARCH)/$(BIN): build-dirs
	@echo "building: $@"
	@$(DOCKER) run                                                             \
	    -ti                                                                    \
	    -u $$(id -u):$$(id -g)                                                 \
	    -v $(ROOT_DIR)/.go:/go                                                 \
	    -v $(ROOT_DIR):/go/src/$(PKG)                                          \
	    -v $(OUTPUT_DIR)/$(ARCH):/go/bin                                       \
	    -v $(OUTPUT_DIR)/$(ARCH):/go/bin/linux_$(ARCH)                         \
	    -v $(ROOT_DIR)/.go/std/$(ARCH):/usr/local/go/pkg/linux_$(ARCH)_static  \
	    -w /go/src/$(PKG)                                                      \
	    $(BUILD_IMAGE)                                                         \
	    /bin/sh -c "                                                           \
	        ARCH=$(ARCH)                                                       \
	        VERSION=$(VERSION)                                                 \
	        PKG=$(PKG)                                                         \
	        ./build/build.sh                                                   \
	    "

DOTFILE_IMAGE = $(subst :,_,$(subst /,_,$(IMAGE))-$(VERSION))

container: .container-$(DOTFILE_IMAGE) container-name
.container-$(DOTFILE_IMAGE): bin/$(ARCH)/$(BIN) Dockerfile.in
	@sed \
	    -e 's|ARG_BIN|$(BIN)|g' \
	    -e 's|ARG_ARCH|$(ARCH)|g' \
	    -e 's|ARG_FROM|$(BASEIMAGE)|g' \
	    Dockerfile.in > .dockerfile-$(ARCH)
	@${DOCKER} build -t $(IMAGE):$(VERSION) -f .dockerfile-$(ARCH) .
	@${DOCKER} images -q $(IMAGE):$(VERSION) > $@

	@${DOCKER} build -t $(IMAGE):latest -f .dockerfile-$(ARCH) .
	@${DOCKER} images -q $(IMAGE):$(VERSION) > $@

container-name:
	@echo "container: $(IMAGE):$(VERSION)"

push: .push-$(DOTFILE_IMAGE) push-name
.push-$(DOTFILE_IMAGE): .container-$(DOTFILE_IMAGE)
ifeq ($(findstring gcr.io,$(REGISTRY)),gcr.io)
	@gcloud docker -- push $(IMAGE):$(VERSION)
else
	@docker push $(IMAGE):$(VERSION)
endif
	@docker images -q $(IMAGE):$(VERSION) > $@

push-name:
	@echo "pushed: $(IMAGE):$(VERSION)"

version:
	@echo $(VERSION)

test:
ifneq ($(SKIP_TESTS), 1)
# go test -i installs compiled packages so they can be reused later
# This speeds up retests.
	go test -i -v $(TEST_PKGS)
	go test $(TEST_PKGS) $(TESTARGS)
endif

.PHONY: all container push test verify update

build-dirs:
	@mkdir -p _output/$(ARCH)
	@mkdir -p .go/src/$(PKG) .go/pkg .go/bin .go/std/$(ARCH)

clean: container-clean bin-clean

container-clean:
	rm -rf .container-* .dockerfile-* .push-*
	$(DOCKER) rmi $(REGISTRY)/$(PROJECT):latest $(REGISTRY)/$(PROJECT):$(VERSION) 2>/dev/null || :

bin-clean:
	rm -rf .go _output