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
local common = import "examples/ksonnet/components/common.jsonnet";
local minio = import "examples/ksonnet/components/minio.jsonnet";

local Deployment = k.apps.v1beta1.deployment;

{
  prereqs(conf):: k.core.v1.list.new([
    common.components.namespace(conf),
    common.components.serviceAccount(conf),
  ]),
  customResources(conf, k8sVersion)::
    local generator =
      if k8sVersion.major >= 1 && k8sVersion.minor >= 7
      then function(resourceNames)
        common.components.customResources.crd(conf, resourceNames)
      else function(resourceNames)
        common.components.customResources.tpr(conf, resourceNames);
    k.core.v1.list.new(
      std.map(generator, conf.customResources.resources)
    ),
  arkDeployment:: {
    local containerName = "ark",
    new(conf, cloudProvider)::
      local deployment =
        if cloudProvider != "azure"
        then $.arkDeployment.nonAzure(conf, cloudProvider)
        else $.arkDeployment.azure(conf);
      k.core.v1.list.new([deployment]),
    nonAzure(conf, cloudProvider)::
      local volumeParams = conf.deployment.volumes[containerName][cloudProvider];
      local attachVolumeMounts = function(container)
        common.functions.attachVolumeMounts(container, volumeParams);
      local attachEnv = function(container)
        common.functions.attachEnv(container, conf.deployment.env[containerName][cloudProvider]);
      common.components.deployment(conf) +
      Deployment.mixin.spec.template.spec.volumes(common.components.volumes.fromSecret(volumeParams)) +
      Deployment.mapContainersWithName(containerName, attachVolumeMounts) +
      Deployment.mapContainersWithName(containerName, attachEnv),
    azure(conf)::
      local attachEnvFrom = function(container)
        common.functions.attachEnvFrom(container, conf.deployment.envFrom[containerName].azure);
      common.components.deployment.baseDeployment(conf) +
      Deployment.mapContainersWithName(containerName, attachEnvFrom),
  },
  arkConfig:: {
    local apiVersion = {apiVersion: "ark.heptio.com/v1"},
    local kind = {kind: "Config"},
    base(spec):: apiVersion + kind + spec,
    new(conf, configParams)::
      local config =
        $.arkConfig.base(conf.spec) +
        $.arkConfig.mixin.name(conf.name) +
        $.arkConfig.mixin.namespace(conf.metadata.namespace) +
        $.arkConfig.mixin.labels(conf.metadata.labels) +
        $.arkConfig.mixin.cloud.backupStorageProvider(configParams) +
        $.arkConfig.mixin.cloud.persistentVolumeProvider(configParams);
      k.core.v1.list.new([config]),
    mixin:: {
      local __metadataMixin(metadata) = {metadata+: metadata},
      local __persistentVolumeProviderMixin(data) = {persistentVolumeProvider+: data},
      local __backupStorageProviderMixin(data) = {backupStorageProvider+: data},
      name(name):: __metadataMixin({name: name}),
      namespace(namespace):: __metadataMixin({namespace: namespace}),
      labels(labels):: __metadataMixin({labels+: labels}),
      cloud:: {
        backupStorageProvider(configParams)::
          local cloudProvider = configParams.cloudProvider;
          local backupParams = std.filter(function(p) p.backupField, configParams.providerParams);
          __backupStorageProviderMixin({
            bucket: configParams.bucket,
            [cloudProvider]:
              if cloudProvider != "minio"
              then std.foldl(function(agg, p) agg + p, backupParams, {})
              else {
                region: "minio",
                availabilityZone: "minio",
                s3ForcePathStyle: true,
                s3Url: "http://minio:9000",
              },
          }),
        persistentVolumeProvider(configParams)::
          local cloudProvider = configParams.cloudProvider;
          local pvParams = std.filter(function(p) !p.backupField, configParams.providerParams);
          if std.length(pvParams) > 0
          then __persistentVolumeProviderMixin({
            [cloudProvider]: std.foldl(function(agg, p) agg + p, configParams.providerParams, {})
          })
          else {},
      },
    },
  },
  optional:: {
    rbac(conf, rbacEnabled)::
      if rbacEnabled == "0"
      then k.core.v1.list.new([])
      else k.core.v1.list.new([
        common.components.clusterRole(conf),
        common.components.clusterRoleBinding(conf),
        common.components.role(conf),
        common.components.roleBinding(conf),
      ]),
    minio(conf, cloudProvider)::
      if cloudProvider == "minio"
      then minio.all(conf)
      else k.core.v1.list.new([]),
  },
}
