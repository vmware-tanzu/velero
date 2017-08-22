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

{
  metadata: {
    namespace: "heptio-ark",
    labels: {component: "ark"},
  },
  customResources: {
    group: "ark.heptio.com",
    version: "v1",
    scope: "Namespaced",
    resources: [
      {
        name: "Backup",
        plural: "backups"
      },
      {
        name: "Schedule",
        plural: "schedules"
      },
      {
        name: "Restore",
        plural: "restores"
      },
      {
        name: "Config",
        plural: "configs"
      },
    ],
  },
  serviceAccount: {
    name: "ark",
  },
  clusterRole: {
    name: "ark",
    rules: [
      { 
        apiGroups: ["*"],
        verbs: ["list", "watch", "create"],
        resources: ["*"],
      },
      {
        apiGroups: ["apiextensions.k8s.io"],
        verbs: ["create"],
        resources: ["customresourcedefinitions"],
      },
    ],
  },
  clusterRoleBinding: {
    name: "ark",
    subjects: [
      {
        kind: "ServiceAccount",
        namespace: $.metadata.namespace,
        name: $.serviceAccount.name,
      },
    ],
    roleRef: {
      kind: "ClusterRole",
      name: $.clusterRole.name,
      apiGroup: "rbac.authorization.k8s.io",
    },
  },
  role: {
    name: "ark",
    rules: [
      {
        apiGroups: [$.customResources.group],
        verbs: ["*"],
        resources: ["*"],
      },
    ],
  },
  roleBinding: $.clusterRoleBinding + {
    roleRef+: {
      kind: "Role",
    },
  },
  deployment: {
    name: "ark",
    replicas: 1,
    containers: [
      { 
        image: "gcr.io/heptio-images/ark:latest",
        name: "ark",
        command: ["ark"],
        args: [
          "server",
          "--logtostderr",
          "--v",
          "4",
        ],     
      },
    ],
    volumes: {
      ark: {
        aws: [
          {
            name: "cloud-credentials",
            secretName: "cloud-credentials",
            mountPath: "/credentials",
          },
        ],
        gcp: $.deployment.volumes.ark.aws,
        minio: $.deployment.volumes.ark.aws,
      },
    },
    env: {
      ark: {
        aws: [
          {
            name: "AWS_SHARED_CREDENTIALS_FILE",
            value: $.deployment.volumes.ark.aws[0].mountPath + "/cloud",
          },
        ],
        gcp: [
          $.deployment.env.ark.aws[0] + {name: "GOOGLE_CREDENTIALS_FILE"},
        ],
        minio: $.deployment.env.ark.aws,
      },
    },
    envFrom: {
      ark: {
        azure: [
          {
            secretRef: {name: "cloud-credentials"},
          },
        ],
      },
    },
  },
}
