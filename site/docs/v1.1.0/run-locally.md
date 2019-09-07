# Run Velero locally in development

## Run Velero locally with a remote cluster

Velero runs against the Kubernetes API server as the endpoint (as per the `kubeconfig` configuration), so both the Velero server and client use the same `client-go` to communicate with Kubernetes. This means the Velero server can be run locally just as functionally as if it was running in the remote cluster.

Step 1) Install Velero in a remote cluster normally. See the "Install" section for your specific provider

Step 2) Scale the Velero deployment down to 0 so it is not simultaneously being run on the remote cluster and potentially causing things to get out of sync

`kubectl scale --replicas=0 deployment velero -n velero`

Step 3) Boot the local Velero server

a) From under the Velero project directory, make and save the binaries locally

`make local`

b) run the server

To run the server locally, use the path according to the binary you need. Example, if you are on a Mac, this would be where to find the binary you just made: `/_output/bin/darwin/amd64/velero`.

Note: Since you are running locally, chances are you will want to run with the log level at debug, so add that flag.

`./_output/bin/darwin/amd64/velero server --log-level debug`