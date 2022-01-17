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

FROM golang:1.17

ARG GOPROXY

ENV GO111MODULE=on
# Use a proxy for go modules to reduce the likelihood of various hosts being down and breaking the build
ENV GOPROXY=${GOPROXY}

# get code-generation tools (for now keep in GOPATH since they're not fully modules-compatible yet)
RUN mkdir -p /go/src/k8s.io
WORKDIR /go/src/k8s.io
RUN git config --global advice.detachedHead false
RUN git clone -b v0.22.2 https://github.com/kubernetes/code-generator

# kubebuilder test bundle is separated from kubebuilder. Need to setup it for CI test.
RUN curl -sSLo envtest-bins.tar.gz https://go.kubebuilder.io/test-tools/1.22.1/linux/amd64 && \
    mkdir /usr/local/kubebuilder && \
    tar -C /usr/local/kubebuilder --strip-components=1 -zvxf envtest-bins.tar.gz

RUN wget --quiet https://github.com/kubernetes-sigs/kubebuilder/releases/download/v3.2.0/kubebuilder_linux_amd64 && \
    mv kubebuilder_linux_amd64 /usr/local/kubebuilder/bin/kubebuilder && \
    chmod +x /usr/local/kubebuilder/bin/kubebuilder

# get controller-tools
RUN go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.7.0

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

# install cosign
COPY --from=gcr.io/projectsigstore/cosign:v1.4.1@sha256:502d5130431e45f28c51d2c24a05ef5ccd3fd916bcc91db0c8bee3a81e09a0bb /ko-app/cosign /usr/local/bin/cosign

# install goreleaser
ARG GORELEASER_VERSION=1.5.0
ARG GORELEASER_SHA=a7bd4f76a495ed91f087dbf86fdbfa5f1bb594e86c1ec953e6b2dd46f596563b
RUN  \
	wget https://github.com/goreleaser/goreleaser/releases/download/v$GORELEASER_VERSION/checksums.txt.pem && \
	GORELEASER_DOWNLOAD_FILE=goreleaser_Linux_x86_64.tar.gz && \
	GORELEASER_DOWNLOAD_URL=https://github.com/goreleaser/goreleaser/releases/download/v${GORELEASER_VERSION}/${GORELEASER_DOWNLOAD_FILE} && \
	cosign verify-blob --cert checksums.txt.pem \
	--signature https://github.com/goreleaser/goreleaser/releases/download/v$GORELEASER_VERSION/checksums.txt.sig \
	https://github.com/goreleaser/goreleaser/releases/download/v$GORELEASER_VERSION/checksums.txt && \
	wget ${GORELEASER_DOWNLOAD_URL} && \
	echo "$GORELEASER_SHA $GORELEASER_DOWNLOAD_FILE" | sha256sum -c - || exit 1 && \
	tar -xzf $GORELEASER_DOWNLOAD_FILE -C /usr/bin/ goreleaser && \
	rm $GORELEASER_DOWNLOAD_FILE && \
	rm checksums.txt.pem && \
	goreleaser -v

# get golangci-lint
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.27.0

# install kubectl
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl
RUN chmod +x ./kubectl
RUN mv ./kubectl /usr/local/bin
