# Build Instructions

The base tag this release is branched from is `v2.1.0`

Create Environment Variables

```
export DOCKER_REPO=<Docker Repository>
export DOCKER_NAMESPACE=<Docker Namespace>
export DOCKER_TAG=v1.8.1-BFS
```

Build and Push Images

```
# Build and push Rancher Backup Operator
git tag -d v1.8.1
git tag  v1.8.1
export DOCKER_CLI_EXPERIMENTAL=enabled
docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
docker buildx rm builder || true
docker buildx create --use --name=builder
docker buildx inspect --bootstrap
REGISTRY=${params.DOCKER_REPO}/${params.DOCKER_NAMESPACE}/${DOCKER_IMAGE_NAME} VERSION=${DOCKER_IMAGE_TAG} BUILDX_PLATFORMS=linux/amd64 BUILDX_OUTPUT_TYPE=registry make container
```