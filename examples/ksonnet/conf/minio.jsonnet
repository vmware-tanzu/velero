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
    labels: {component: "minio"},
  },
  deployment: {
    name: "minio",
    replicas: 1,
    strategy: {type: "Recreate"},
    containers: [
      { 
        image: "minio/minio:latest",
        name: "minio",
        ports: [{containerPort: $.service.ports[0].targetPort}],
        imagePullPolicy: "IfNotPresent",
        args: [
          "server",
          "/storage",
        ],   
      },
    ],
    volumes: {
      minio: [
        {
          name: "storage",
          hostPath: "/tmp/minio",
          mountPath: "/storage",
        },
      ],
    },
    env: {
      minio: [
        {
          name: "MINIO_ACCESS_KEY",
          value: "minio",
        },
        {
          name: "MINIO_SECRET_KEY",
          value: "minio123",
        },
      ],
    },
  },
  service: {
    name: "minio",
    ports: [
      {
        port: 9000,
        targetPort: 9000,
        protocol: "TCP",
      },
    ],
    selector: $.metadata.labels,
    type: "ClusterIP",
  },
  secret: {
    name: "cloud-credentials",
    stringData: {
      cloud: "[default]\naws_access_key_id = minio\naws_secret_access_key = minio123\n",
    },
  },
  job: {
    name: "minio-setup",
    restartPolicy: "OnFailure",
    containers: [
      { 
        image: "minio/mc:latest",
        name: "mc",
        imagePullPolicy: "IfNotPresent",
        command: [
          "/bin/sh",
          "-c",
          "mc config host add ark http://minio:9000 minio minio123 && mc mb -p ark/ark",
        ],   
      },
    ],
  },
}