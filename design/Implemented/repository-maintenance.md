# Design for repository maintenance job

## Abstract
This design proposal aims to decouple repository maintenance from the Velero server by launching a maintenance job when needed, to mitigate the impact on the Velero server during backups.

## Background
During backups, Velero performs periodic maintenance on the repository. This operation may consume significant CPU and memory resources in some cases, leading to potential issues such as the Velero server being killed by OOM. This proposal addresses these challenges by separating repository maintenance from the Velero server.

## Goals
1. **Independent Repository Maintenance**: Decouple maintenance from Velero's main logic to reduce the impact on the Velero server pod.

2. **Configurable Resources Usage**: Make the resources used by the maintenance job configurable.

3. **No API Changes**: Retain existing APIs and workflow in the backup repository controller.

## Non Goals
We have lots of concerns over parallel maintenance, which will increase the complexity of our design currently.

 - Non-blocking maintenance job: it may conflict with updating the same `backuprepositories` CR when parallel maintenance.

 - Maintenance job concurrency control: there is no one suitable mechanism in Kubernetes to control the concurrency of different jobs.

 - Parallel maintenance: Maintaining the same repo by multiple jobs at the same time would have some compatible cases that some providers may not support.

Unfortunately, parallel maintenance is currently not a priority because of the concerns above, improving maintenance efficiency is not the primary focus at this stage.

## High-Level Design
1. **Add Maintenance Subcommand**: Introduce a new Velero server subcommand for repository maintenance.

2. **Create Jobs by Repository Manager**: Modify the backup repository controller to create a maintenance job instead of directly calling the multiple chain calls for Kopia or Restic maintenance.

3. **Update Maintenance Job Result in BackupRepository CR**: Retrieve the result of the maintenance job and update the status of the `BackupRepository` CR accordingly.

4. **Add Setting for Maintenance Job**: Introduce a configuration option to set maintenance jobs, including resource limits (CPU and memory), keeping the latest N maintenance jobs for each repository.

## Detailed Design

### 1. Add Maintenance sub-command

The CLI command will be added to the Velero CLI, the command is designed for use in a pod of maintenance jobs. 

Our CLI command is designed as follows:
```shell
$ velero repo-maintenance --repo-name $repo-name --repo-type $repo-type --backup-storage-location $bsl
```

Compared with other CLI commands, the maintenance command is used in a pod of maintenance jobs not for user use, and the job should show the result of maintenance after finish.

Here we will write the error message into one specific file which could be read by the maintenance job.

on the whole, we record two kinds of logs:

- one is the log output of the intermediate maintenance process: this log could be retrieved via the Kubernetes API server, including the error log.

- one is the result of the command which could indicate whether the execution is an error or not: the result could be redirected to a file that the maintenance job itself could read, and the file only contains the error message.

we will write the error message into the `/dev/termination-log` file if execution is failed.

The main maintenance logic would be using the repository provider to do the maintenance.

```golang
func checkError(err error, file *os.File) {
	if err != nil {
		if err != context.Canceled {
			if _, errWrite := file.WriteString(fmt.Sprintf("An error occurred: %v", err)); errWrite != nil {
				fmt.Fprintf(os.Stderr, "Failed to write error to termination log file: %v\n", errWrite)
			}
			file.Close()
			os.Exit(1) // indicate the command executed failed
		}
	}
}

func (o *Options) Run(f veleroCli.Factory) {
	logger := logging.DefaultLogger(o.LogLevelFlag.Parse(), o.FormatFlag.Parse())
	logger.SetOutput(os.Stdout)

	errorFile, err := os.Create("/dev/termination-log")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create termination log file: %v\n", err)
		return
	}
	defer errorFile.Close()
	...

  err = o.runRepoPrune(cli, f.Namespace(), logger)
	checkError(err, errorFile)
	...
}

func (o *Options) runRepoPrune(cli client.Client, namespace string, logger logrus.FieldLogger) error {
  ...
	var repoProvider provider.Provider
	if o.RepoType == velerov1api.BackupRepositoryTypeRestic {
		repoProvider = provider.NewResticRepositoryProvider(credentialFileStore, filesystem.NewFileSystem(), logger)
	} else {
		repoProvider = provider.NewUnifiedRepoProvider(
			credentials.CredentialGetter{
				FromFile:   credentialFileStore,
				FromSecret: credentialSecretStore,
			}, o.RepoType, cli, logger)
	}
  ...

	err = repoProvider.BoostRepoConnect(context.Background(), para)
	if err != nil {
		return errors.Wrap(err, "failed to boost repo connect")
	}

	err = repoProvider.PruneRepo(context.Background(), para)
	if err != nil {
		return errors.Wrap(err, "failed to prune repo")
	}
	return nil
}
```

### 2. Create Jobs by Repository Manager
Currently, the backup repository controller will call the repository manager to do the `PruneRepo`, and Kopia or Restic maintenance is then finally called through multiple chain calls.

We will keep using the `PruneRepo` function in the repository manager, but we cut off the multiple chain calls by creating a maintenance job.

The job definition would be like below:
```yaml
apiVersion: v1
items:
- apiVersion: batch/v1
  kind: Job
  metadata:
    # labels or affinity or topology settings would inherit from the velero deployment
    labels:
      # label the job name for later list jobs by name
      job-name: nginx-example-default-kopia-pqz6c
    name: nginx-example-default-kopia-pqz6c
    namespace: velero
  spec:
    # Not retry it again
    backoffLimit: 1
    # Only have one job one time
    completions: 1
    # Not parallel running job
    parallelism: 1
    template:
      metadata:
        labels:
          job-name: nginx-example-default-kopia-pqz6c
        name: kopia-maintenance-job
      spec:
        containers:
        # arguments for repo maintenance job
        - args:
          - repo-maintenance
          - --repo-name=nginx-example
          - --repo-type=kopia
          - --backup-storage-location=default
	  # inherit from Velero server
          - --log-level=debug
          command:
          - /velero
          # inherit environment variables from the velero deployment
          env:
          - name: AZURE_CREDENTIALS_FILE
            value: /credentials/cloud
          # inherit image from the velero deployment
          image: velero/velero:main
          imagePullPolicy: IfNotPresent
          name: kopia-maintenance-container
	  # resource limitation set by Velero server configuration
	  # if not specified, it would apply best effort resources allocation strategy
          resources: {}
          # error message would be written to /dev/termination-log
          terminationMessagePath: /dev/termination-log
          terminationMessagePolicy: File
          # inherit volume mounts from the velero deployment
          volumeMounts:
          - mountPath: /credentials
            name: cloud-credentials
        dnsPolicy: ClusterFirst
        restartPolicy: Never
        schedulerName: default-scheduler
        securityContext: {}
        # inherit service account from the velero deployment
        serviceAccount: velero
        serviceAccountName: velero
        volumes:
        # inherit cloud credentials from the velero deployment
        - name: cloud-credentials
          secret:
            defaultMode: 420
            secretName: cloud-credentials
	# ttlSecondsAfterFinished set the job expired seconds
	ttlSecondsAfterFinished: 86400
  status:
	# which contains the result after maintenance
	message: ""
	lastMaintenanceTime: ""
```

Now, the backup repository controller will call the repository manager to create one maintenance job and wait for the job to complete. The Kopia or Restic maintenance multiple chains are called by the job.

### 3. Update the Result of the Maintenance Job into BackupRepository CR

The backup repository controller will update the result of the maintenance job into the backup repository CR.

For how to get the result of the maintenance job we could refer to [here](https://kubernetes.io/docs/tasks/debug/debug-application/determine-reason-pod-failure/#writing-and-reading-a-termination-message).

After the maintenance job is finished, we could get the result of maintenance by getting the terminated message from the related pod:

```golang
func GetContainerTerminatedMessage(pod *v1.Pod) string {
	...
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.LastTerminationState.Terminated != nil {
			return containerStatus.LastTerminationState.Terminated.Message
		}
	}
	...
	return ""
}
```
Then we could update the status of backupRepository CR with the message.

### 4. Add Setting for Resource Usage of Maintenance
Add one configuration for setting the resource limit of maintenance jobs as below:
```shell
    velero server --maintenance-job-cpu-request $cpu-request --maintenance-job-mem-request $mem-request --maintenance-job-cpu-limit $cpu-limit --maintenance-job-mem-limit $mem-limit
```
Our default value is 0, which means we don't limit the resources, and the resource allocation strategy would be [best effort](https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/#besteffort).

### 5. Automatic Cleanup for Finished Maintenance Jobs
Add configuration for clean up maintenance jobs:

- keep-latest-maintenance-jobs: the number of keeping latest maintenance jobs for each repository.

	```shell
		velero server --keep-latest-maintenance-jobs $num
	```

We would check and keep the latest N jobs after a new job is finished.
```golang
func deleteOldMaintenanceJobs(cli client.Client, repo string, keep int) error {
	// Get the maintenance job list by label
	jobList := &batchv1.JobList{}
	err := cli.List(context.TODO(), jobList, client.MatchingLabels(map[string]string{RepositoryNameLabel: repo}))
	if err != nil {
		return err
	}

	// Delete old maintenance jobs
	if len(jobList.Items) > keep {
		sort.Slice(jobList.Items, func(i, j int) bool {
			return jobList.Items[i].CreationTimestamp.Before(&jobList.Items[j].CreationTimestamp)
		})
		for i := 0; i < len(jobList.Items)-keep; i++ {
			err = cli.Delete(context.TODO(), &jobList.Items[i], client.PropagationPolicy(metav1.DeletePropagationBackground))
			if err != nil {
				return err
			}
		}
	}

	return nil
}
```

### 6 Velero Install with Maintenance Options
All the above maintenance options should be supported by Velero install command.

### 7. Observability and Debuggability
Some monitoring metrics are added for backup repository maintenance:
- repo_maintenance_total
- repo_maintenance_success_total
- repo_maintenance_failed_total
- repo_maintenance_duration_seconds

We will keep the latest N maintenance jobs for each repo, and users can get the log from the job. the job log level inherent from the Velero server setting.

Also, we would integrate maintenance job logs and `backuprepositories` CRs into `velero debug`.

Roughly, the process is as follows:
1. The backup repository controller will check the BackupRepository request in the queue periodically.

2. If the maintenance period of the repository checked by `runMaintenanceIfDue` in `Reconcile` is due, then the backup repository controller will call the Repository manager to execute `PruneRepo`

3. The `PruneRepo` of the Repository manager will create one maintenance job, the resource limitation, environment variables, service account, images, etc. would inherit from the Velero server pod. Also, one clean up TTL would be set to maintenance job.

4. The maintenance job will execute the Velero maintenance command, wait for maintaining to finish and write the maintenance result into the terminationMessagePath file of the related pod.

5. Kubernetes could show the result in the status of the pod by reading the termination message in the pod.

6. The backup repository controller will wait for the maintenance job to finish and read the status of the maintenance job, then update the message field and phase in the status of `backuprepositories` CR accordingly.

6. Clean up old maintenance jobs and keep only N latest for each repository.

### 8. Codes Refinement
Once `backuprepositories` CR status is modified, the CR would re-queue to be reconciled, and re-execute logics in reconcile shortly not respecting the re-queue frequency configured by `repoSyncPeriod`.
For one abnormal scenario if the maintenance job fails, the status of `backuprepositories` CR would be updated and the CR will re-queue immediately, if the new maintenance job still fails, then it will re-queue again, making the logic of `backuprepositories` CR re-queue like a dead loop.

So we change the Predicates logic in Controller manager making it only re-queue if the Spec of `backuprepositories` CR is changed.

```golang
  ctrl.NewControllerManagedBy(mgr).For(&velerov1api.BackupRepository{}, builder.WithPredicates(kube.SpecChangePredicate{}))
```

This change would bring the behavior different from the previous, errors that occurred in the maintenance job would retry in the next reconciliation period instead of retrying immediately.

## Prospects for Future Work
Future work may focus on improving the efficiency of Velero maintenance through non-blocking parallel modes. Potential areas for enhancement include:

**Non-blocking Mode**: Explore the implementation of a non-blocking mode for parallel maintenance to enhance overall efficiency.

**Concurrency Control**: Investigate mechanisms for better concurrency control of different maintenance jobs.

**Provider Support for Parallel Maintenance**: Evaluate the feasibility of parallel maintenance for different providers and address any compatibility issues.

**Efficiency Improvements**: Investigate strategies to optimize maintenance efficiency without compromising reliability.

By considering these areas, future iterations of Velero may benefit from enhanced parallelization and improved resource utilization during repository maintenance.
