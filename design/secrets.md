# Design proposal template (replace with your proposal's title)

Currently, Velero only supports a single AIM access credential secret per location provider/plugin. Velero creates and stores the plugin credential secret under the hard-coded key `secret.cloud-credentials.data.cloud`.

This makes it so switching from one plugin to another necessitates overriding the existing credential secret with the appropriate one for the new plugin.

## Goals

- To allow Velero to create and store multiple secrets for provider credentials
- To improve the UX for configuring the velero deployment with multiple plugins/providers, and corresponding IAM secrets.

## Non Goals

- To make any change except what's necessary to handle multiple credentials

## Detailed Design

- the name of the flag changes from `secret-file` to `--credentials-file`. 

- The `velero plugin (create|set) --credentials-file` will be a map of provider name as a key, and the path to the file as a value. This way, we can have multiple credential secrets and each secret per provider/plugin will be unique.

See discussion https://github.com/vmware-tanzu/velero/pull/2259#discussion_r384700723 for the two items below.

- The `velero backup-location (create|set)` will have a new `--credentials mapStringString` flag which sets the name of the corresponding credentials secret for a provider. Format is provider:credentials-secret-name.

- The `velero snapshot-location (create|set)` will have a new `--credentials mapStringString` flag which sets the list of name of the corresponding credentials secret for providers. Format is (provider1:credentials-secret-name1,provider2:credentials-secret-name2,...).

Note that for this logic to work we must have a controller loop checking for when a corresponding secret is present before marking the BSL/VSL as ready.


#### Examples of mounting secrets and environment variables

With the changes proposed in the previous section (Credentials and secrets), the resulting deployment `yaml` would look like below:

AWS
```
   spec:
     containers:
        volumeMounts:
        - mountPath: /credentials
          name: cloud-credentials-aws
      - args:
        - server
        name: velero
        env:
        - name: AWS_SHARED_CREDENTIALS_FILE
          value: /credentials/cloud
     volumes:
      - name: cloud-credentials
        secret:
          secretName: cloud-credentials
```

Azure
```
   spec:
     containers:
        volumeMounts:
        - mountPath: /credentials
          name: cloud-credentials-azure
      - args:
        - server
        name: velero
        env:
        - name: AZURE_SHARED_CREDENTIALS_FILE
          value: /credentials/cloud
     volumes:
      - name: cloud-credentials-azure
        secret:
          secretName: cloud-credentials-azure
```


## Alternatives Considered

N/A

## Security Considerations

N/A
