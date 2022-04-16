# Copyright 2020 the Velero contributors.
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
FROM --platform=$BUILDPLATFORM ghcr.io/oracle/oraclelinux:7-slim as builder-env

#ENV GOPATH=/root/go
#ENV PATH=$PATH:/usr/local/go/bin/
#ENV PATH=$PATH:${GOPATH}/bin/
#
#RUN mkdir -p ${GOPATH}/src && mkdir -p ${GOPATH}/bin && \
#    microdnf update -y && \
#    microdnf clean all && \
#    rm -rf /var/cache/yum/*
#
#RUN microdnf install git wget tar gzip bzip2
#
#RUN wget https://go.dev/dl/go1.17.5.linux-amd64.tar.gz \
#    && rm -rf /usr/local/go \
#    && tar -C /usr/local -xzf go1.17.5.linux-amd64.tar.gz

RUN yum-config-manager --enable ol7_optional_latest && \
    yum-config-manager --enable ol7_addons &&\
    yum update -y && \
    # software collections repo needed for git 2.x on OL7
    yum-config-manager --add-repo=http://yum.oracle.com/repo/OracleLinux/OL7/SoftwareCollections/x86_64 && \
    yum-config-manager --enable ol7_developer_golang117 &&\
    yum install -y rh-git227 oracle-golang-release-el7 golang-1.17.5 bzip2 && \
    # Set up needed to ensure git 2.27 from rh-git227 is on the path
    ln /opt/rh/rh-git227/enable /etc/profile.d/git.sh && \
    source /etc/profile.d/git.sh && \
    git version

ARG GOPROXY
ARG PKG
ARG VERSION
ARG GIT_SHA
ARG GIT_TREE_STATE
ARG REGISTRY

ENV CGO_ENABLED=0 \
    GO111MODULE=on \
    GOPROXY=${GOPROXY} \
    LDFLAGS="-X ${PKG}/pkg/buildinfo.Version=${VERSION} -X ${PKG}/pkg/buildinfo.GitSHA=${GIT_SHA} -X ${PKG}/pkg/buildinfo.GitTreeState=${GIT_TREE_STATE} -X ${PKG}/pkg/buildinfo.ImageRegistry=${REGISTRY}"

WORKDIR /go/src/github.com/vmware-tanzu/velero

COPY . /go/src/github.com/vmware-tanzu/velero

FROM --platform=$BUILDPLATFORM builder-env as builder

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG PKG
ARG BIN
ARG RESTIC_VERSION

ENV GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    GOARM=${TARGETVARIANT}

RUN mkdir -p /output/usr/bin && \
    bash ./hack/download-restic.sh && \
    export GOARM=$( echo "${GOARM}" | cut -c2-) && \
    go build -o /output/${BIN} \
    -ldflags "${LDFLAGS}" ${PKG}/cmd/${BIN}

FROM ghcr.io/oracle/oraclelinux:7-slim
COPY --from=builder /output /
#RUN  microdnf update -y && \
#     microdnf clean all && \
#     rm -rf /var/cache/yum/* \
RUN yum update -y
USER nonroot:nonroot

