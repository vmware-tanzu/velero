# Copyright 2018, 2019, 2020, 2021 the Velero contributors.
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

FROM bitnami/golang:1.15

ARG GOPROXY

ENV GO111MODULE=on
# Use a proxy for go modules to reduce the likelihood of various hosts being down and breaking the build
ENV GOPROXY=${GOPROXY}

# Perform base actions first for things that will not change often so we don't
# need to create new image layers as often.
RUN apt-get update && apt-get install -y \
  unzip \
  curl \
  git \
  bash \
  && rm -rf /var/lib/apt/lists/*

# get golangci-lint
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.27.0

# get goimports (the revision is pinned so we don't indiscriminately update, but the particular commit
# is not important)
RUN go get golang.org/x/tools/cmd/goimports@11e9d9cc0042e6bd10337d4d2c3e5d9295508e7d

# get controller-tools
RUN go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0

# get code-generation tools (for now keep in GOPATH since they're not fully modules-compatible yet)
WORKDIR /root
RUN mkdir -p /go/src/k8s.io && \
    git config --global advice.detachedHead false && \
    git clone --depth 1 -b v0.18.4 https://github.com/kubernetes/code-generator /go/src/k8s.io/code-generator

RUN wget --quiet https://github.com/kubernetes-sigs/kubebuilder/releases/download/v2.3.1/kubebuilder_2.3.1_linux_amd64.tar.gz && \
    tar -zxvf kubebuilder_2.3.1_linux_amd64.tar.gz && \
    mv kubebuilder_2.3.1_linux_amd64 /usr/local/kubebuilder && \
    chmod +x /usr/local/kubebuilder && \
    export PATH=$PATH:/usr/local/kubebuilder/bin && \
    rm kubebuilder_2.3.1_linux_amd64.tar.gz

# get protoc compiler and golang plugin
RUN wget --quiet https://github.com/protocolbuffers/protobuf/releases/download/v3.9.1/protoc-3.9.1-linux-x86_64.zip && \
    unzip protoc-3.9.1-linux-x86_64.zip && \
    mv bin/protoc /usr/bin/protoc && \
    chmod +x /usr/bin/protoc && \
    rm protoc-3.9.1-linux-x86_64.zip && \
    go get github.com/golang/protobuf/protoc-gen-go@v1.0.0

# get goreleaser
RUN wget --quiet https://github.com/goreleaser/goreleaser/releases/download/v0.120.8/goreleaser_Linux_x86_64.tar.gz && \
    tar xvf goreleaser_Linux_x86_64.tar.gz && \
    mv goreleaser /usr/bin/goreleaser && \
    chmod +x /usr/bin/goreleaser && \
    rm goreleaser_Linux_x86_64.tar.gz

# install kubectl
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && \
    chmod +x ./kubectl && \
    mv ./kubectl /usr/local/bin
