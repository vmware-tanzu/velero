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

FROM golang:1.12

RUN mkdir -p /go/src/k8s.io
WORKDIR /go/src/k8s.io
RUN git config --global advice.detachedHead false
RUN git clone -b kubernetes-1.17.0 https://github.com/kubernetes/code-generator
RUN git clone -b kubernetes-1.17.0 https://github.com/kubernetes/apimachinery 
# Use a proxy for go modules to reduce the likelihood of various hosts being down and breaking the build
ENV GOPROXY=https://proxy.golang.org
# vendor code-generator go modules to be compatible with pre-1.15
WORKDIR  /go/src/k8s.io/code-generator
# Don't use ENV here because we don't want to disable modules for subsequent commands
RUN GO111MODULE=on go mod vendor
RUN mkdir -p /go/src/sigs.k8s.io
WORKDIR /go/src/sigs.k8s.io
RUN git clone -b v0.2.2 https://github.com/kubernetes-sigs/controller-tools
WORKDIR /go/src/sigs.k8s.io/controller-tools
RUN GO111MODULE=on go mod vendor
RUN go get golang.org/x/tools/cmd/goimports
WORKDIR /go/src/golang.org/x/tools
RUN git checkout 40a48ad93fbe707101afb2099b738471f70594ec
RUN go install ./cmd/goimports
RUN echo chmod -R a+w /go
WORKDIR /root
RUN apt-get update && apt-get install -y unzip
RUN wget --quiet https://github.com/protocolbuffers/protobuf/releases/download/v3.9.1/protoc-3.9.1-linux-x86_64.zip && \
    unzip protoc-3.9.1-linux-x86_64.zip && \
    mv bin/protoc /usr/bin/protoc && \
    chmod +x /usr/bin/protoc
RUN mkdir -p /go/src/github.com/golang
WORKDIR /go/src/github.com/golang
RUN git clone -b v1.0.0 https://github.com/golang/protobuf
WORKDIR /go/src/github.com/golang/protobuf
RUN go install ./protoc-gen-go
RUN wget --quiet https://github.com/goreleaser/goreleaser/releases/download/v0.120.8/goreleaser_Linux_x86_64.tar.gz && \
    tar xvf goreleaser_Linux_x86_64.tar.gz && \
    mv goreleaser /usr/bin/goreleaser && \
    chmod +x /usr/bin/goreleaser
