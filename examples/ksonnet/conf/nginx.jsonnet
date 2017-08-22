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
    namespace: "nginx-example",
    labels: {app: "nginx"},
  },
  persistentVolumeClaim: {
    name: "nginx-logs",
    accessModes: ["ReadWriteOnce"],
    resources: {
      requests: {storage: "50Mi"},
    },
  },
  deployment: {
    name: "nginx-deployment",
    replicas: 1,
    containers: [
      { 
        image: "nginx:1.7.9",
        name: "nginx",
        ports: [{containerPort: $.service.ports[0].targetPort}],
      },
    ],
    volumes: {
      nginx: [
        {
          name: "nginx-logs",
          pvcName: $.persistentVolumeClaim.name,
          mountPath: "/var/log/nginx",
        },
      ],
    },
  },
  service: {
    name: "my-nginx",
    ports: [
      {
        port: 80,
        targetPort: 80,
      },
    ],
    selector: $.metadata.labels,
    type: "LoadBalancer",
  },
}