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
GOTARGET = github.com/heptio/$(PROJECT)
OUTPUT_DIR = $(ROOT_DIR)/_output
BIN_DIR = $(OUTPUT_DIR)/bin

# docker related vars
DOCKER ?= docker
REGISTRY ?= gcr.io/heptio-images
BUILD_IMAGE ?= $(REGISTRY)/golang:1.8-alpine3.6
# go build -i installs compiled packages so they can be reused later.
# This speeds up recompiles.
BUILDCMD = go build -i -v -ldflags "-X $(GOTARGET)/pkg/buildinfo.Version=$(VERSION) -X $(GOTARGET)/pkg/buildinfo.DockerImage=$(REGISTRY)/$(PROJECT)"
BUILDMNT = /go/src/$(GOTARGET)
EXTRA_MNTS ?=

# test related vars
TESTARGS ?= -timeout 60s
TEST_PKGS ?= ./cmd/... ./pkg/...
SKIP_TESTS ?=

# what we're building
BINARIES = ark

local: $(BINARIES)

$(BINARIES):
	mkdir -p $(BIN_DIR)
	$(BUILDCMD) -o $(BIN_DIR)/$@ $(GOTARGET)/cmd/$@

test:
ifneq ($(SKIP_TESTS), 1)
# go test -i installs compiled packages so they can be reused later
# This speeds up retests.
	go test -i -v $(TEST_PKGS)
	go test $(TEST_PKGS) $(TESTARGS)
endif

verify:
ifneq ($(SKIP_TESTS), 1)
	${ROOT_DIR}/hack/verify-generated-docs.sh
	${ROOT_DIR}/hack/verify-generated-clientsets.sh
	${ROOT_DIR}/hack/verify-generated-listers.sh
	${ROOT_DIR}/hack/verify-generated-informers.sh
endif

update:
	${ROOT_DIR}/hack/update-generated-docs.sh
	${ROOT_DIR}/hack/update-generated-clientsets.sh
	${ROOT_DIR}/hack/update-generated-listers.sh
	${ROOT_DIR}/hack/update-generated-informers.sh

all: cbuild container

cbuild:
	$(DOCKER) run --rm -v $(ROOT_DIR):$(BUILDMNT) $(EXTRA_MNTS) -w $(BUILDMNT) -e SKIP_TESTS=$(SKIP_TESTS) $(BUILD_IMAGE) /bin/sh -c 'make local verify test'

container: cbuild
	$(DOCKER) build -t $(REGISTRY)/$(PROJECT):latest -t $(REGISTRY)/$(PROJECT):$(VERSION) .

container-local: $(BINARIES)
	$(DOCKER) build -t $(REGISTRY)/$(PROJECT):latest -t $(REGISTRY)/$(PROJECT):$(VERSION) .

push:
	docker -- push $(REGISTRY)/$(PROJECT):$(VERSION)

.PHONY: all local container cbuild push test verify update $(BINARIES)

clean:
	rm -rf $(OUTPUT_DIR)
	$(DOCKER) rmi $(REGISTRY)/$(PROJECT):latest $(REGISTRY)/$(PROJECT):$(VERSION) 2>/dev/null || :
