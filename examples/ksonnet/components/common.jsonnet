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

local Namespace = k.core.v1.namespace;
local ServiceAccount = k.core.v1.serviceAccount;
local ClusterRole = k.rbac.v1beta1.clusterRole;
local ClusterRoleBinding = k.rbac.v1beta1.clusterRoleBinding;
local Role = k.rbac.v1beta1.role;
local RoleBinding = k.rbac.v1beta1.roleBinding;
local TPR = k.extensions.v1beta1.thirdPartyResource;
local Deployment = k.apps.v1beta1.deployment;
local Service = k.core.v1.service;
local PVC = k.core.v1.persistentVolumeClaim;
local Job = k.batch.v1.job;
local Secret = k.core.v1.secret;

local Container = Deployment.mixin.spec.template.spec.containersType;
local Volume = Deployment.mixin.spec.template.spec.volumesType;

{
  components:: {
    namespace(conf)::
      Namespace.new() + 
      Namespace.mixin.metadata.name(conf.metadata.namespace) +
      Namespace.mixin.metadata.labels(conf.metadata.labels),
    serviceAccount(conf)::
      ServiceAccount.new() + 
      ServiceAccount.mixin.metadata.name(conf.serviceAccount.name) + 
      ServiceAccount.mixin.metadata.namespace(conf.metadata.namespace) + 
      ServiceAccount.mixin.metadata.labels(conf.metadata.labels),
    clusterRole(conf)::
      ClusterRole.new() +
      ClusterRole.mixin.metadata.name(conf.clusterRole.name) +
      ClusterRole.mixin.metadata.labels(conf.metadata.labels) +
      ClusterRole.rules(conf.clusterRole.rules),
    clusterRoleBinding(conf)::
      ClusterRoleBinding.new() +
      ClusterRoleBinding.mixin.metadata.name(conf.clusterRoleBinding.name) +
      ClusterRoleBinding.mixin.metadata.labels(conf.metadata.labels) +
      ClusterRoleBinding.subjects(conf.clusterRoleBinding.subjects) +
      ClusterRoleBinding.mixin.roleRef.mixinInstance(conf.clusterRoleBinding.roleRef),
    role(conf)::
      Role.new() +
      Role.mixin.metadata.name(conf.role.name) +
      Role.mixin.metadata.labels(conf.metadata.labels) +
      Role.rules(conf.role.rules),
    roleBinding(conf)::
      RoleBinding.new() +
      RoleBinding.mixin.metadata.name(conf.roleBinding.name) +
      RoleBinding.mixin.metadata.labels(conf.metadata.labels) +
      RoleBinding.subjects(conf.roleBinding.subjects) +
      RoleBinding.mixin.roleRef.mixinInstance(conf.roleBinding.roleRef),
    pvc(conf, storageClassName)::
      PVC.new() +
      PVC.mixin.metadata.name(conf.persistentVolumeClaim.name) +
      PVC.mixin.metadata.namespace(conf.metadata.namespace) +
      PVC.mixin.metadata.labels(conf.metadata.labels) +
      PVC.mixin.spec.storageClassName(storageClassName) +
      PVC.mixin.spec.accessModes(conf.persistentVolumeClaim.accessModes) +
      PVC.mixin.spec.resources.mixinInstance(conf.persistentVolumeClaim.resources),
    deployment(conf)::
      Deployment.new(
        conf.deployment.name, 
        conf.deployment.replicas, 
        conf.deployment.containers, 
        conf.metadata.labels) +
      Deployment.mixin.metadata.namespace(conf.metadata.namespace) +
      Deployment.mixin.metadata.labels(conf.metadata.labels),
    service(conf)::
      Service.new(
        conf.service.name, 
        conf.service.selector, 
        conf.service.ports) +
      Service.mixin.metadata.namespace(conf.metadata.namespace) +
      Service.mixin.metadata.labels(conf.metadata.labels) +
      Service.mixin.spec.type(conf.service.type),
    secret(conf)::
      Secret.new() +
      Secret.mixin.metadata.name(conf.secret.name) +
      Secret.mixin.metadata.namespace(conf.metadata.namespace) +
      Secret.mixin.metadata.labels(conf.metadata.labels) +
      Secret.stringData(conf.secret.stringData),
    job(conf)::
      Job.new() +
      Job.mixin.metadata.name(conf.job.name) +
      Job.mixin.metadata.namespace(conf.metadata.namespace) +
      Job.mixin.metadata.labels(conf.metadata.labels) +
      Job.mixin.spec.template.spec.containers(conf.job.containers) +
      Job.mixin.spec.template.spec.restartPolicy(conf.job.restartPolicy),
    volumes:: {
      fromPvc(volumeParams)::
        std.map(
          function(v) Volume.fromPersistentVolumeClaim(v.name, v.pvcName),
          volumeParams),
      fromSecret(volumeParams)::
        std.map(
          function(v) Volume.name(v.name) + Volume.mixin.secret.secretName(v.secretName),          
          volumeParams),
    },
    customResources:: {
      crd(conf, resourceNames)::
        $.components.crd.new() +
        $.components.crd.mixin.labels(conf.metadata.labels) +
        $.components.crd.mixin.version(conf.customResources.version) +
        $.components.crd.mixin.scope(conf.customResources.scope) +
        $.components.crd.mixin.groupWithNames(conf.customResources.group, resourceNames.name, resourceNames.plural),
      tpr(conf, resourceNames)::
        TPR.new() +
        TPR.mixin.metadata.name(resourceNames.plural + "." + conf.customResources.group) +
        TPR.mixin.metadata.labels(conf.metadata.labels) +
        TPR.versions([{name: conf.customResources.version}]),
    },
    crd:: {
      local apiVersion = {apiVersion: "apiextensions.k8s.io/v1beta1"},
      local kind = {kind: "CustomResourceDefinition"},
      new():: apiVersion + kind,
      mixin:: {
        local __metadataMixin(metadata) = {metadata+: metadata},
        local __specMixin(spec) = {spec+: spec},
        labels(labels):: __metadataMixin({labels+: labels}),
        version(version):: __specMixin({version: version}),
        scope(scope):: __specMixin({scope: scope}),
        groupWithNames(group, name, plural)::
          local metadataName = plural + "." + group;
          local names = {
            plural: plural,
            kind: name,
          };
          __metadataMixin({name: metadataName}) + __specMixin({group: group, names: names})
      },
    },
  },
  functions:: {
    attachVolumeMounts(container, volumeParams)::
      local attachVolumeMount = function(aggregate, v)
        local volumeMount = Deployment.mixin.spec.template.spec.containersType.volumeMountsType.new(v.name, v.mountPath);
        aggregate + Container.volumeMounts(volumeMount);
      std.foldl(attachVolumeMount, volumeParams, container),
    attachEnv(container, envParams)::
      local attachSingleEnv = function(aggregate, e)
        local env = Deployment.mixin.spec.template.spec.containersType.env(e);
        aggregate + env;
      std.foldl(attachSingleEnv, envParams, container),
    attachEnvFrom(container, envFromParams)::
      local attachSingleEnvFrom = function(aggregate, e)
        local envFrom = Deployment.mixin.spec.template.spec.containersType.envFrom(e);
        aggregate + envFrom;
      std.foldl(attachSingleEnvFrom, envFromParams, container),
  },
}
