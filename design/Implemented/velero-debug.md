# `velero debug` command for gathering troubleshooting information

## Abstract
To simplify the communication between velero users and developers, this document proposes the `velero debug` command to generate a tarball including the logs needed for debugging.

Github issue: https://github.com/vmware-tanzu/velero/issues/675

## Background
Gathering information to troubleshoot a Velero deployment is currently spread across multiple commands, and is not very efficient. Logs for the Velero server itself are accessed via a kubectl logs command, while information on specific backups or restores are accessed via a Velero subcommand. Restic logs are even more complicated to retrieve, since one must gather logs for every instance of the daemonset, and there’s currently no good mechanism to locate which node a particular restic backup ran against.  
A dedicated subcommand can lower this effort and reduce back-and-forth between user and developer for collecting the logs.


## Goals
- Enable efficient log collection for Velero and associated components, like plugins and restic.

## Non Goals
- Collecting logs for components that do not belong to velero such as storage service.
- Automated log analysis.

## High-Level Design
With the introduction of the new command `velero debug`, the command would download all of the following information:
- velero deployment logs
- restic DaemonSet logs
- plugin logs 
- All the resources in the group `velero.io` that are created such as:
    - Backup
    - Restore
    - BackupStorageLocation
    - PodVolumeBackup
    - PodVolumeRestore
    - *etc ...*
- Log of the backup and restore, if specified in the param

A project called `crash-diagnostics` (or `crashd`) (https://github.com/vmware-tanzu/crash-diagnostics)  implements the Kubernetes API queries and provides Starlark scripting language to abstract details, and collect the information into a local copy.   It can be used as a standalone CLI executing a Starlark script file.
With the capabilities of embedding files in Go 1.16, we can define a Starlark script gathering the necessary information, embed the script at build time, then the velero debug command will invoke `crashd`, passing in the script’s text contents.

## Detailed Design
### Triggering the script
The Starlark script to be called by crashd:

```python
def capture_backup_logs(cmd, namespace):
    if args.backup:
        log("Collecting log and information for backup: {}".format(args.backup))
        backupDescCmd = "{} --namespace={} backup describe {} --details".format(cmd, namespace, args.backup)
        capture_local(cmd=backupDescCmd, file_name="backup_describe_{}.txt".format(args.backup))
        backupLogsCmd = "{} --namespace={} backup logs {}".format(cmd, namespace, args.backup)
        capture_local(cmd=backupLogsCmd, file_name="backup_{}.log".format(args.backup))
def capture_restore_logs(cmd, namespace):
    if args.restore:
        log("Collecting log and information for restore: {}".format(args.restore))
        restoreDescCmd = "{} --namespace={} restore describe {} --details".format(cmd, namespace, args.restore)
        capture_local(cmd=restoreDescCmd, file_name="restore_describe_{}.txt".format(args.restore))
        restoreLogsCmd = "{} --namespace={} restore logs {}".format(cmd, namespace, args.restore)
        capture_local(cmd=restoreLogsCmd, file_name="restore_{}.log".format(args.restore))

ns = args.namespace if args.namespace else "velero"
output = args.output if args.output else "bundle.tar.gz"
cmd = args.cmd if args.cmd else "velero"
# Working dir for writing during script execution
crshd = crashd_config(workdir="./velero-bundle")
set_defaults(kube_config(path=args.kubeconfig, cluster_context=args.kubecontext))
log("Collecting velero resources in namespace: {}". format(ns))
kube_capture(what="objects", namespaces=[ns], groups=['velero.io'])
capture_local(cmd="{} version -n {}".format(cmd, ns), file_name="version.txt")
log("Collecting velero deployment logs in namespace: {}". format(ns))
kube_capture(what="logs", namespaces=[ns])
capture_backup_logs(cmd, ns)
capture_restore_logs(cmd, ns)
archive(output_file=output, source_paths=[crshd.workdir])
log("Generated debug information bundle: {}".format(output))
```
The sample command to trigger the script via crashd:
```shell
./crashd run ./velero.cshd --args 
'backup=harbor-backup-2nd,namespace=velero,basedir=,restore=,kubeconfig=/home/.kube/minikube-250-224/config,output='
```
To trigger the script in `velero debug`, in the package `pkg/cmd/cli/debug` a struct `option` will be introduced
```go
type option struct {
	// currCmd the velero command
	currCmd string
	// workdir for crashd will be $baseDir/velero-debug
	baseDir string
	// the namespace where velero server is installed
	namespace string
	// the absolute path for the log bundle to be generated
	outputPath string
	// the absolute path for the kubeconfig file that will be read by crashd for calling K8S API
	kubeconfigPath string
	// the kubecontext to be used for calling K8S API
	kubeContext string
	// optional, the name of the backup resource whose log will be packaged into the debug bundle
	backup string
	// optional, the name of the restore resource whose log will be packaged into the debug bundle
	restore string
	// optional, it controls whether to print the debug log messages when calling crashd
	verbose bool
}
```
The code will consolidate the input parameters and execution context of the `velero` CLI to form the option struct, which can be transformed into the `argsMap` that can be used when calling the func `exec.Execute` in `crashd`:
https://github.com/vmware-tanzu/crash-diagnostics/blob/v0.3.4/exec/executor.go#L17

## Alternatives Considered
The collection could be done via the kubernetes client-go API, but such integration is not necessarily trivial to implement, therefore, `crashd` is preferred approach

## Security Considerations
- The starlark script will be embedded into the velero binary, and the byte slice will be passed to the `exec.Execute` func directly, so there’s little risk that the script will be modified before being executed.

## Compatibility
As the `crashd` project evolves the behavior of the internal functions used in the Starlark script may change.  We’ll ensure the correctness of the script via regular E2E tests.


## Implementation
1. Bump up to use Go v1.16 to compile velero
2. Embed the starlark script
3. Implement the `velero debug` sub-command to call the script
4. Add E2E test case

## Open Questions
- **Command dependencies:** In the Starlark script, for collecting version info and backup logs, it calls the `velero backup logs` and `velero version`, which makes the call stack like velero debug -> crashd -> velero xxx.  We need to make sure this works under different PATH settings.
- **Progress and error handling:** The log collection may take a relatively long time, log messages should be printed to indicate the progress when different items are being downloaded and packaged.  Additionally, when an error happens, `crashd` may omit some errors, so before the script is executed we'll do some validation and make sure the `debug` command fail early if some parameters are incorrect.
