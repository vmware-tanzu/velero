# Copyright 2018, 2019, 2020 the Velero contributors.
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

FROM golang:1.15

ARG GOPROXY

ENV GO111MODULE=on
# Use a proxy for go modules to reduce the likelihood of various hosts being down and breaking the build
ENV GOPROXY=${GOPROXY}

# get code-generation tools (for now keep in GOPATH since they're not fully modules-compatible yet)
RUN mkdir -p /go/src/k8s.io
WORKDIR /go/src/k8s.io
RUN git config --global advice.detachedHead false
RUN git clone -b v0.18.4 https://github.com/kubernetes/code-generator

RUN wget --quiet https://github.com/kubernetes-sigs/kubebuilder/releases/download/v2.3.1/kubebuilder_2.3.1_linux_amd64.tar.gz && \
    tar -zxvf kubebuilder_2.3.1_linux_amd64.tar.gz && \
    mv kubebuilder_2.3.1_linux_amd64 /usr/local/kubebuilder && \
    chmod +x /usr/local/kubebuilder && \
    export PATH=$PATH:/usr/local/kubebuilder/bin && \
    rm kubebuilder_2.3.1_linux_amd64.tar.gz

# get controller-tools
RUN go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0

# get goimports (the revision is pinned so we don't indiscriminately update, but the particular commit
# is not important)
RUN go get golang.org/x/tools/cmd/goimports@11e9d9cc0042e6bd10337d4d2c3e5d9295508e7d

# get protoc compiler and golang plugin
WORKDIR /root
RUN apt-get update && apt-get install -y unzip
RUN wget --quiet https://github.com/protocolbuffers/protobuf/releases/download/v3.9.1/protoc-3.9.1-linux-x86_64.zip && \
    unzip protoc-3.9.1-linux-x86_64.zip && \
    mv bin/protoc /usr/bin/protoc && \
    chmod +x /usr/bin/protoc
RUN go get github.com/golang/protobuf/protoc-gen-go@v1.0.0

# get goreleaser
RUN wget --quiet https://github.com/goreleaser/goreleaser/releases/download/v0.120.8/goreleaser_Linux_x86_64.tar.gz && \
    tar xvf goreleaser_Linux_x86_64.tar.gz && \
    mv goreleaser /usr/bin/goreleaser && \
    chmod +x /usr/bin/goreleaser

# get golangci-lint
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.27.0

# install kubectl
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl
RUN chmod +x ./kubectl
RUN mv ./kubectl /usr/local/bin
