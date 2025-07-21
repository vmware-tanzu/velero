---
title: Velero developer local install and debug with vscode 
layout: docs
---

The following example sets up Velero server and client locally for use with the Visual Studio Code.

## Get the latest Velero source code
```bash
mkdir VELERO_DEV && pushd VELERO_DEV
mkdir bin
git clone https://github.com/vmware-tanzu/velero.git
pushd velero
go build ./cmd/velero
make local
popd
cp velero/_output/bin/linux/amd64/velero bin/
export PATH=$PATH:$PWD/bin
which velero
```

**NOTE:** Ensure that the velero client binary that was just built is indeed the one in the $PATH

## Setup a storage plugin
In this example you will use aws as the storage plugin.

```bash
# From the VELERO_DEV directory
mkdir plugins
mkdir plugin_src && pushd plugin_src
git clone https://github.com/vmware-tanzu/velero-plugin-for-aws.git
cd velero-plugin-for-aws/
make local
cp _output/bin/linux/amd64/velero-plugin-for-aws ../../plugins/
popd
```

## Setup s3 object storage

1. Create a Velero specific credentials file (`credentials-velero`) in your Velero directory:

    ```bash
    cat <<"EOF" > credentials-velero
    [default]
    aws_access_key_id = minio
    aws_secret_access_key = minio123
    EOF
    ```

1. Start the server and the local storage service. In the Velero directory, run:

    ```bash
    kubectl apply -f velero/examples/minio/00-minio-deployment.yaml
    ```
1. Get the logs from the Minio container
    ```bash
    # get the ip and port information from the minio logs
    kubectl logs -f <pod/minio> -n velero
    ```

**WARNING:**
The minio storage url must be accessible directly from your laptop [1].
There are many ways to ensure minio is accessible from configuring an ingress route to running minio locally in a container.  A quick and easy way to just get going is to forward the minio ports:

[1] [expose-minio-outside-your-cluster-with-a-service](https://velero.io/docs/v1.10/contributions/minio/#expose-minio-outside-your-cluster-with-a-service)

```bash
kubectl port-forward <pod/minio> <admin port> 9000 -n velero
```

## Install Velero  

From the minio logs get the ip and port, ensure <admin port> and <api port> are forwarded.

Set a variable for the Minio endpoint
```bash
export MINIO_ENDPOINT=http://<minio ip>:9000
```
Create a script to install Velero
```bash
cat <<EOF > install_velero.sh
velero install \\
  --provider aws \\
  --plugins velero/velero-plugin-for-aws:v1.6.0 \\
  --bucket velero \\
  --secret-file ./credentials-velero \\
  --use-volume-snapshots=false \\
  --backup-location-config region=minio,s3ForcePathStyle="true",s3Url=$MINIO_ENDPOINT
EOF
```

Ensure the script has the right settings for your install, and execute
```bash
chmod +x install_velero.sh
./install_velero.sh
```

Verify the install
```bash
kubectl get all -n velero
```
Check the version
```bash
velero version
```
Check the backup location
```bash
velero backup-location get
```


## Running Velero locally

By following the above instructions you have successfully installed Velero and configured storage.  Now you need to scale down the replicas and run the server code locally.

List all the resources in the velero namespace to verify
```bash
kubectl get replicaset.apps -n velero
```
Now scale down
```bash
kubectl scale --replicas=0 deployment velero -n velero
```
Ensure Velero has scaled down to zero pods running
```bash
kubectl get replicaset.apps -n velero
```
Example output
```
NAME                DESIRED   CURRENT   READY   AGE
minio-796cc55795    1         1         1       4m31s
velero-58f6678ccf   0         0         0       65s
```

**Note:**
When starting the Velero server please ensure you have another console open with your KUBECONFIG set and PATH set with velero.  This will assist in your validation


Start the Velero server from the VELERO_DEV directory
```bash
AWS_SHARED_CREDENTIALS_FILE=credentials-velero velero server --log-level=debug --kubeconfig=$KUBECONFIG --namespace=velero --plugin-dir=plugins
```

In another console, verify the server is running properly by running `velero version`.  The command should return with:
```bash
$ velero version
Client:
	Version: main
	Git commit: 09098f879cd07df33388d9f50339dbbefa9cfd5c
Server:
	Version: main
```

## Recreate the backup storage location for a local development environment:

Now that the Velero server is running locally you need to update the backup location.

Get the backup location
```bash
$ velero backup-location get
```
```
NAME      PROVIDER   BUCKET/PREFIX   PHASE       LAST VALIDATED                  ACCESS MODE   DEFAULT
default   aws        velero          Unavailable   2022-12-09 13:25:15 -0700 MST   ReadWrite     true
```

The minio ip address used is no longer valid from your laptop and must be deleted.
```bash
velero backup-location delete default
```

Create a new backup location with localhost as the end point
```bash
velero backup-location create default --provider aws --bucket velero --config region=minio,s3ForcePathStyle="true",s3Url=http://localhost:9000 --credential=cloud-credentials=cloud
```

Verify the backup location
```bash
$ velero backup-location get
```
``` 
NAME      PROVIDER   BUCKET/PREFIX   PHASE       LAST VALIDATED                  ACCESS MODE   DEFAULT
default   aws        velero          Available   2022-12-14 13:25:43 -0700 MST   ReadWrite     true
```


## Running Debug Mode in Visual Code

The Velero server should now be confirmed to work and can be shutdown.  Now prepare for executing the Velero server in the Visual Code IDE.

**NOTE**: Your current directory should VELERO_DEV
```bash
cp $KUBECONFIG .
```

Launch Visual Studio Code

* Open the VELERO_DEV directory
* Click `Run` --> `Add Configuration` --> `GO launch`

* Overwrite the configuration with the following:
```
{
    // ${workspaceFolder} - the path of the folder opened in VS Code
    // if $workspaceFolder = VELERO_DEV this config will work as written
    // if $workspaceFolder = velero the program path = "${workspaceFolder}/cmd/velero/velero.go"
    
    // Use IntelliSense to learn about possible attributes.
    // Hover to view descriptions of existing attributes.
    // For more information, visit: https://go.microsoft.com/fwlink/?linkid=830387
    "version": "0.2.0",
    "configurations": [

        {
            "name": "Attach to Process",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${workspaceFolder}/velero/cmd/velero/velero.go",
            "env": {
                "KUBECONFIG": "${workspaceFolder}/kubeconfig",
            },
            "args": ["server", "--log-level=debug", "--kubeconfig=${env:KUBECONFIG}", "--namespace=velero", "--plugin-dir=${workspaceFolder}/plugins/"]
        } 
    ]
}

```

In Visual Studio Code

* Click `Run`  --> `Debug`
    

* Victory is yours
\o/
