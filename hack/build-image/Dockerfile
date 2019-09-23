# Copyright 2018, 2019 the Velero contributors.
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

RUN mkdir -p /go/src/k8s.io && \
    cd /go/src/k8s.io && \
    git config --global advice.detachedHead false && \
    git clone -b kubernetes-1.15.3 https://github.com/kubernetes/code-generator && \
    git clone -b kubernetes-1.15.3 https://github.com/kubernetes/apimachinery && \
    # vendor code-generator go modules to be compatible with pre-1.15
    cd /go/src/k8s.io/code-generator && GO111MODULE=on go mod vendor && \
    go get -d sigs.k8s.io/controller-tools/cmd/controller-gen && \
    cd /go/src/sigs.k8s.io/controller-tools && GO111MODULE=on go mod vendor && \
    go get golang.org/x/tools/cmd/goimports && \
    cd /go/src/golang.org/x/tools && \
    git checkout 40a48ad93fbe707101afb2099b738471f70594ec && \
    go install ./cmd/goimports && \
    echo chmod -R a+w /go && \
    # download & install protoc
    apt-get update && apt-get install -y unzip && \
    wget --quiet https://github.com/protocolbuffers/protobuf/releases/download/v3.9.1/protoc-3.9.1-linux-x86_64.zip && \
    unzip protoc-3.9.1-linux-x86_64.zip && \
    mv bin/protoc /usr/bin/protoc && \
    chmod +x /usr/bin/protoc && \
    # download & install protoc-gen-go plugin (v1.0.0)
    mkdir -p /go/src/github.com/golang && \
    cd /go/src/github.com/golang && \
    git config --global advice.detachedHead false && \
    git clone -b v1.0.0 https://github.com/golang/protobuf && \
    cd /go/src/github.com/golang/protobuf && \
    go install ./protoc-gen-go
