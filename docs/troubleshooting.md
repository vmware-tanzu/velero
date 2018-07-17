# Troubleshooting

These tips can help you troubleshoot known issues. If they don't help, you can [file an issue][4], or talk to us on the [#ark-dr channel][25] on the Kubernetes Slack server. 

Some general commands for troubleshooting that may be helpful:

* `ark backup describe <backupName>` - describe the details of a backup
* `ark backup logs <backupName>` - fetch the logs for this specific backup. Useful for viewing failures and warnings, including resources that could not be backed up.
* `ark restore describe <restoreName>` - describe the details of a restore
* `ark restore logs <restoreName>` - fetch the logs for this specific restore. Useful for viewing failures and warnings, including resources that could not be restored.
* `kubectl logs deployment/ark -n heptio-ark` - fetch the logs of the Ark server pod. This provides the output of the Ark server processes.

## Getting ark debug logs

You can increase the verbosity of the Ark server by editing your Ark deployment to look like this:


```
kubectl edit deployment/ark -n heptio-ark
...
   containers:
     - name: ark
       image: gcr.io/heptio-images/ark:latest
       command:
         - /ark
       args:
         - server
         - --log-level # Add this line
         - debug       # Add this line
...
```


* [Debug installation/setup issues][2]

* [Debug restores][1]

[1]: debugging-restores.md
[2]: debugging-install.md
[4]: https://github.com/heptio/ark/issues
[25]: https://kubernetes.slack.com/messages/ark-dr
