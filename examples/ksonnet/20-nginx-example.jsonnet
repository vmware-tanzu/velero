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

local k = import "ksonnet.beta.2/k.libsonnet";
local nginxConf = import "examples/ksonnet/conf/nginx.jsonnet";
local common = import "examples/ksonnet/components/common.jsonnet";
local Deployment = k.apps.v1beta1.deployment;

local env = {
  cloudProvider: std.extVar("CLOUD_PROVIDER"),
  pvEnabled: std.extVar("PV_ENABLED"),
};

local parts = {
  namespace::
    common.components.namespace(nginxConf),
  basicDeployment::
    common.components.deployment(nginxConf),
  pvDeployment::
    local containerName = "nginx";
    local attachVolumeMounts = function(container)
      common.functions.attachVolumeMounts(container, nginxConf.deployment.volumes[containerName]);

    common.components.deployment(nginxConf) +
    Deployment.mixin.spec.template.spec.volumes(common.components.volumes.fromPvc(nginxConf.deployment.volumes[containerName])) +
    Deployment.mapContainersWithName(containerName, attachVolumeMounts),
  pvc::
    local storageClassName =
      if env.cloudProvider == "aws"
      then "gp2"
      else "standard";
    common.components.pvc(nginxConf, storageClassName),
  service::
    common.components.service(nginxConf),
};

if env.pvEnabled == "1"
then k.core.v1.list.new([
  parts.namespace,
  parts.pvc,
  parts.pvDeployment,
  parts.service,
])
else k.core.v1.list.new([
  parts.namespace,
  parts.basicDeployment,
  parts.service,
])
