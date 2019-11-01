# Code Standards

## Adding a changelog

Whenever making any change that impacts how a user uses Velero, after you open the PR, use the PR number to create a file in the format `<PR NUMBER>-<GH USERNAME>` and add it to this directory:

https://github.com/vmware-tanzu/velero/tree/master/changelogs/unreleased

Add that to the PR.

## Code

- Log messages are capitalized.

- Error messages are kept lower-cased.

- Wrap/add a stack only to errors that are being directly returned from non-velero code, such as an API call to the Kubernetes server.

    ```bash
    errors.WithStack(err)
    ```

- Prefer to use the utilities in the Kubernetes package [`sets`](https://godoc.org/github.com/kubernetes/apimachinery/pkg/util/sets).

    ```bash
    k8s.io/apimachinery/pkg/util/sets
    ```

## Imports

For imports, we use the following convention:

<group><version><api | client | informer | ...>

So, for example:

    import (
        corev1api "k8s.io/api/core/v1"
    	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
    	corev1listers "k8s.io/client-go/listers/core/v1"
        
        velerov1api ""github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
        velerov1client "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1"
    )

## Mocks

We use a package to generate mocks for our interfaces.

Example: if you want to change this mock: https://github.com/vmware-tanzu/velero/blob/master/pkg/restic/mocks/restorer.go

Run:

```bash
go get github.com/vektra/mockery/.../
cd pkg/restic
mockery -name=Restorer
```

Might need to run `make update` to update the imports.
