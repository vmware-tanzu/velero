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

local arkConf = import "examples/ksonnet/conf/ark.jsonnet";
local configConf = import "examples/ksonnet/conf/config.jsonnet";
local minioConf = import "examples/ksonnet/conf/minio.jsonnet";
local arkLib = import "examples/ksonnet/components/ark.jsonnet";

local cloudProviderParams = {
  aws: [
    { key: "region", envName: "AWS_REGION", pvOnly: false, },
    { key: "availabilityZone", envName: "AWS_AVAILABILITY_ZONE", pvOnly: true, },
  ],
  azure: [
    { key: "location", envName: "AZURE_LOCATION", pvOnly: true, },
    { key: "apiTimeout", envName: "AZURE_API_TIMEOUT", pvOnly: true, },
  ],
  gcp: [
    { key: "project", envName: "GCP_PROJECT", pvOnly: true, },
    { key: "zone", envName: "GCP_ZONE", pvOnly: true, },
  ],
  minio: [],
};

local env = {
  local cloudProvider = std.extVar("CLOUD_PROVIDER"),
  local pvEnabled = std.extVar("PV_ENABLED"),
  arkConfig: {
    local filterPVParams = function(hash)
      pvEnabled == "1" || (pvEnabled == "0" && !hash.pvOnly),
    local getEnv = function(hash)
      {[hash.key]: std.extVar(hash.envName), backupField:: !hash.pvOnly },
    cloudProvider: cloudProvider,
    bucket: if cloudProvider == "minio" then "" else std.extVar("BUCKET"),
    providerParams: std.filterMap(filterPVParams, getEnv, cloudProviderParams[cloudProvider]),
  },
  k8sVersion: {
    local versions = std.map(std.parseInt, std.split(std.extVar("K8S_VERSION"), ".")),
    major: versions[0],
    minor: versions[1],
  },
  pvEnabled: pvEnabled,
  rbacEnabled: std.extVar("RBAC_ENABLED"),
};

arkLib.optional.rbac(arkConf, env.rbacEnabled) +
arkLib.customResources(arkConf, env.k8sVersion) +
arkLib.optional.minio(minioConf, env.arkConfig.cloudProvider) +
arkLib.arkConfig.new(configConf, env.arkConfig) +
arkLib.arkDeployment.new(arkConf, env.arkConfig.cloudProvider)
