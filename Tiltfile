# -*- mode: Python -*-

k8s_yaml([
'config/crd/bases/velero.io_backups.yaml',
'config/crd/bases/velero.io_backupstoragelocations.yaml', 
'config/crd/bases/velero.io_deletebackuprequests.yaml',   
'config/crd/bases/velero.io_downloadrequests.yaml',   
'config/crd/bases/velero.io_podvolumebackups.yaml',  
'config/crd/bases/velero.io_podvolumerestores.yaml',  
'config/crd/bases/velero.io_resticrepositories.yaml',
'config/crd/bases/velero.io_restores.yaml',    
'config/crd/bases/velero.io_schedules.yaml', 
'config/crd/bases/velero.io_serverstatusrequests.yaml',  
'config/crd/bases/velero.io_volumesnapshotlocations.yaml', 
])

# default values
settings = {
   "default_registry": "",
   "enable_restic": False,
   "create_backup_locations": False, 
   "setup-minio": False, 
}

# global settings
settings.update(read_json(
    "tilt-resources/tilt-settings.json",
    default = {},
))

k8s_yaml(kustomize('tilt-resources'))
k8s_yaml('tilt-resources/deployment.yaml')
if settings.get("enable_restic"):
    k8s_yaml('tilt-resources/restic.yaml')
if settings.get("create_backup_locations"):
    k8s_yaml('tilt-resources/velero_v1_backupstoragelocation.yaml')
if settings.get("setup-minio"):
    k8s_yaml('examples/minio/00-minio-deployment.yaml', allow_duplicates=True)

# By default, Tilt automatically allows Minikube, Docker for Desktop, Microk8s, Red Hat CodeReady Containers, Kind, K3D, and Krucible.
allow_k8s_contexts(settings.get("allowed_contexts"))
default_registry(settings.get("default_registry"))
local_goos = str(local("go env GOOS", quiet=True, echo_off=True)).strip()

tilt_helper_dockerfile_header = """
# Tilt image
FROM golang:1.15.3 as tilt-helper
# Support live reloading with Tilt
RUN wget --output-document /restart.sh --quiet https://raw.githubusercontent.com/windmilleng/rerun-process-wrapper/master/restart.sh  && \
    wget --output-document /start.sh --quiet https://raw.githubusercontent.com/windmilleng/rerun-process-wrapper/master/start.sh && \
    chmod +x /start.sh && chmod +x /restart.sh
"""

additional_docker_helper_commands = """
RUN wget -qO- https://dl.k8s.io/v1.19.2/kubernetes-client-linux-amd64.tar.gz | tar xvz
RUN wget -qO- https://get.docker.com | sh
"""

additional_docker_build_commands = """
COPY --from=tilt-helper /usr/bin/docker /usr/bin/docker
COPY --from=tilt-helper /go/kubernetes/client/bin/kubectl /usr/bin/kubectl
"""

##############################
# Setup Velero
##############################

# Set up a local_resource build of the Velero binary. The binary is written to _tiltbuild/velero.
local_resource(
    "velero_server_binary",
    cmd = 'cd ' + '.' + ';mkdir -p _tiltbuild;PKG=. BIN=velero GOOS=linux GOARCH=amd64 VERSION=main GIT_TREE_STATE=dirty OUTPUT_DIR=_tiltbuild ./hack/build.sh',
    deps = ["cmd", "internal", "pkg"],
    ignore = ["pkg/cmd"],
)

local_resource(
    "velero_local_binary",
    cmd = 'cd ' + '.' + ';mkdir -p _tiltbuild/local;PKG=. BIN=velero GOOS=' + local_goos + ' GOARCH=amd64 VERSION=main GIT_TREE_STATE=dirty OUTPUT_DIR=_tiltbuild/local ./hack/build.sh',
    deps = ["internal", "pkg/cmd"],
)

tilt_dockerfile_header = """
FROM gcr.io/distroless/base:debug as tilt
WORKDIR /
COPY --from=tilt-helper /start.sh .
COPY --from=tilt-helper /restart.sh .
COPY velero .
"""

dockerfile_contents = "\n".join([
    tilt_helper_dockerfile_header,
    additional_docker_helper_commands,
    tilt_dockerfile_header,
    additional_docker_build_commands,
])

 # Set up an image build for Velero. The live update configuration syncs the output from the local_resource
 # build into the container.
docker_build(
    ref = "velero/velero",
    context = "_tiltbuild/",
    dockerfile_contents = dockerfile_contents,
    target = "tilt",
    entrypoint=["sh", "/start.sh", "/velero"],
    live_update=[
        sync("./_tiltbuild/velero", "/velero"),
        run("sh /restart.sh"),
    ])

##############################
# Setup plugins
##############################

providers = {}

def load_provider_tiltfiles():
    provider_repos = settings.get("provider_repos", [])

    for repo in provider_repos:
        file = repo + "/tilt-provider.json"
        provider_details = read_json(file, default = {})
        if type(provider_details) != type([]):
            provider_details = [provider_details]
        for item in provider_details:
            provider_name = item["name"]
            provider_config = item["config"]
            if "context" in provider_config:
                provider_config["context"] = repo + "/" + provider_config["context"]
            else:
                provider_config["context"] = repo
            if "go_main" not in provider_config:
                provider_config["go_main"] = "main.go"
            providers[provider_name] = provider_config

# Enable only provider plugins listed in 'enable_providers' in tilt-settings.json (by provider's name)
def enable_providers():
    user_enable_providers = settings.get("enable_providers", [])
    for name in user_enable_providers:
        enable_provider(name)

# Configures a provider by doing the following:
#
# 1. Enables a local_resource go build of the provider's local binary
# 2. Configures a docker build for the provider, with live updating of the local binary
def enable_provider(name):
    p = providers.get(name)
    plugin_name = p.get("plugin_name")

    # Note: we need a distro other than base:debug to get a shell to do a copy of the plugin binary
    tilt_dockerfile_header = """
    FROM ubuntu:focal as tilt
    WORKDIR /
    COPY --from=tilt-helper /start.sh .
    COPY --from=tilt-helper /restart.sh .
    COPY """ + plugin_name + """ .
    """

    dockerfile_contents = "\n".join([
        tilt_helper_dockerfile_header,
        additional_docker_helper_commands,
        tilt_dockerfile_header,
        additional_docker_build_commands,
    ])

    

    context = p.get("context")
    go_main = p.get("go_main", "main.go")

    live_reload_deps = []
    for d in p.get("live_reload_deps", []):
        live_reload_deps.append(context + "/" + d)

    # Set up a local_resource build of the plugin binary. The main.go path must be provided via go_main option. The binary is written to _tiltbuild/<NAME>.
    local_resource(
        name + "_plugin",
        cmd = 'cd ' + context + ';mkdir -p _tiltbuild;PKG=' + context + ' BIN=' + go_main + ' GOOS=linux GOARCH=amd64 OUTPUT_DIR=_tiltbuild ./hack/build.sh',
        deps = live_reload_deps,
    )

    # Set up an image build for the plugin. The live update configuration syncs the output from the local_resource
    # build into the init container, and that restarts the Velero container.
    docker_build(
        ref = p.get("image"),
        context = context + "/_tiltbuild/",
        dockerfile_contents = dockerfile_contents,
        target = "tilt",
        entrypoint=["/bin/bash", "-c", "cp /" + plugin_name + " /target/."],
        live_update=[
            sync(context + "/_tiltbuild/" + plugin_name, "/" + plugin_name),
        ]
    )

##############################
# Start
#############################

load_provider_tiltfiles()

enable_providers()