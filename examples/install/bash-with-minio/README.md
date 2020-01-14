# Auto Deployment (minio + velero)

This script is created to automatically install velero on Kubernetes.  
First, it deploys minio as deployment and service on Kubernetes.  
Then, it deploys velero server by executing velero CLI (install).  

## Tested environment
ubuntu 18.04.3 LTS  
Kubernetes v1.15.3  
Docker 19.03.5  


## Usage
### Install:
`./velero.sh i`

### Uninstall:
`./velero.sh u`
