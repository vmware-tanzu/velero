# Multi-arch Build and Windows Build Support

## Background

At present, Velero images could be built for linux-amd64 and linux-arm64. We need to support other platforms, i.e., windows-amd64.  
At present, for linux image build, we leverage Buildkit's `--platform` option to create the image manifest list in one build call. However, it is a limited way and doesn't fully support all multi-arch scenarios. Specifically, since the build is done in one call with the same parameters, it is impossbile to build images with different configurations (e.g., Windows build requires a different Dockerfile).   
At present, Velero by default build images locally, or no image or manifest is pushed to registry. However, docker doesn't support multi-arch build locally. We need to clarify the behavior of local build.    

## Goals
- Refactor the `make container` process to fully support multi-arch build
- Add Windows build to the existing build process
- Clarify the behavior of local build with multi-arch build capabilities
- Don't change the pattern of the final image tag to be used by users

## Non-Goals
- There may be some workarounds to make the multi-arch image/manifest fully available locally. These workarounds will not be adopted, so local build always build single-arch images

## Local Build

For local build, two values of `--output` parameter for `docker buildx build` are supported:
- `docker`: a docker format image is built, but the image is only built for the platform (`<os>/<arch>`) as same as the building env. E.g., when building from linux-amd64 env, a single manifest of linux-amd64 is created regardless how the input parameters are configured.  
- `tar`: one or more images are built as tarballs according to the input platform (`<os>/<arch>`) parameters. Specifically, one tarball is generated for each platform. The build process is the same with the `Build Separate Manifests` of `Push Build` as detailed below. Merely, the `--output` parameter diffs, as `type=tar;dest=<tarball generated path>`. The tarball is generated to the `_output` folder and named with the platform info, e.g., `_output/velero-main-linux-amd64.tar`.  

## Push Build

For push build, the `--output` parameter for `docker buildx build` is always `registry`. And build will go according to the input parameters and create multi-arch manifest lists.    

### Step 1: Build Separate Manifests

Instead of specifying multiple platforms (`<os>/<arch>`) to `--platform` option, we add multiple `container-%` targets in Makefile and each target builds one platform representively.  

The goal here is to build multiple manifests through the multiple targets. However, `docker buildx build` by default creates a manifest list even though there is only one element in `--platform`. Therefore, two flags `--provenance=false` and `--sbom=false` will be set additionally to force `docker buildx build` to create manifests.  

Each manifest has a unique tag, the OS type and arch is added to the tag, in the pattern `$(REGISTRY)/$(BIN):$(VERSION)-$(OS)-$(ARCH)`. For example, `velero/velero:main-linux-amd64`.  

All the created manifests will be pushed to registry so that the all-in-one manifest list could be created.  

### Step 2: Create All-In-One Manifest List

The next step is to create a manifest list to include all the created manifests. This could be done by `docker manifest create` command, the tags created and pushed at Step 1 are passed to this command.  
A tag is also created for the manifest list, in the pattern `$(REGISTRY)/$(BIN):$(VERSION)`. For example, `velero/velero:main`.  

### Step 3: Push All-In-One Manifest List

The created manifest will be pushed to registry by command `docker manifest push`.  

## Input Parameters

Below are the input parameters that are configurable to meet different build purposes during Dev and release cycle:
- BUILD_OUTPUT_TYPE: the type of output for the build, i.e., `docker`, `tar`, `registry`, while `docker` and `tar` is for local build; `registry` means push build. Default value is `docker`  
- BUILD_OS: which types of OS should be built for. Multiple values are accepted, e.g., `linux,windows`. Default value is `linux`  
- BUILD_ARCH: which types of architecture should be built for. Multiple values are accepted, e.g., `amd64,arm64`. Default value is `amd64`  

## Windows Build

Windows container images vary from Windows OS versions, e.g., `ltsc2022` for Windows server 2022 and `1809` for Windows server 2019. Images for different OS versions should be built separately.  
Therefore, separate build targets are added for each OS version, like `container-windows-%`.  
For the same reason, a new input parameter is added, `BUILD_WINDOWS_VERSION`. The default value is `ltsc2022`. Windows server 2022 is the only base image we will deliver officially, Windows server 2019 is not supported. In future, we may need to support Windows server 2025 base image.  
For local build to tar, the Windows OS version is also added to the name of the tarball, e.g., `_output/velero-main-windows-ltsc2022-amd64.tar`.  

At present, Windows container image only supports `amd64` as the architecture, so `BUILD_ARCH` is ignored for Windows.  

The Windows manifests need to be annotated with os type, arch, and os version. This will be done through `docker manifest annotate` command.  

## Use Malti-arch Images

In order to use the images, the manifest list's tag should be provided to `velero install` command or helm, the individual manifests are covered by the manifest list. During launch time, the container engine will load the right image to the container according to the platform of the running node.  

## Build Samples

**Local build to docker**
```
make container
```
The built image could be listed by `docker image ls`.  

**Local build for linux-amd64 and windows-amd64 to tar**
```
BUILDX_OUTPUT_TYPE=tar BUILD_OS=linux,windows make container
```
Under `_output` directory, below files are generated:  
```
velero-main-linux-amd64.tar
velero-main-windows-ltsc2022-amd64.tar
``` 

**Local build for linux-amd64, linux-arm64 and windows-amd64 to tar**
```
BUILDX_OUTPUT_TYPE=tar BUILD_OS=linux,windows BUILD_ARCH=amd64,arm64 make container
```
Under `_output` directory, below files are generated:  
```
velero-main-linux-amd64.tar
velero-main-linux-arm64.tar
velero-main-windows-ltsc2022-amd64.tar
```

**Push build for linux-amd64 and windows-amd64**  
Prerequisite: login to registry, e.g., through `docker login`  
```
BUILDX_OUTPUT_TYPE=registry REGISTRY=<registry> BUILD_OS=linux,windows make container
```
Nothing is available locally, in the registry 3 tags are available:
```
velero/velero:main
velero/velero:main-windows-ltsc2022-amd64
velero/velero:main-linux-amd64
```

**Push build for linux-amd64, linux-arm64 and windows-amd64**  
Prerequisite: login to registry, e.g., through `docker login` 
```
BUILDX_OUTPUT_TYPE=registry REGISTRY=<registry> BUILD_OS=linux,windows BUILD_ARCH=amd64,arm64 make container
```
Nothing is available locally, in the registry 4 tags are available:
```
velero/velero:main
velero/velero:main-windows-ltsc2022-amd64
velero/velero:main-linux-amd64
velero/velero:main-linux-arm64
```
