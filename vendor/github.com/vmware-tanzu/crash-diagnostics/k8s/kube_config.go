// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
	"github.com/vladimirvivien/gexe"
)

// FetchWorkloadConfig...
func FetchWorkloadConfig(clusterName, clusterNamespace, mgmtKubeConfigPath string) (string, error) {
	var filePath string
	cmdStr := fmt.Sprintf(`kubectl get secrets/%s-kubeconfig --template '{{.data.value}}' --namespace=%s --kubeconfig %s`, clusterName, clusterNamespace, mgmtKubeConfigPath)
	p := gexe.StartProc(cmdStr)
	if p.Err() != nil {
		return filePath, fmt.Errorf("kubectl get secrets failed: %s: %s", p.Err(), p.Result())
	}

	f, err := ioutil.TempFile(os.TempDir(), fmt.Sprintf("%s-workload-config", clusterName))
	if err != nil {
		return filePath, errors.Wrap(err, "Cannot create temporary file")
	}
	filePath = f.Name()
	defer f.Close()

	base64Dec := base64.NewDecoder(base64.StdEncoding, p.Out())
	if _, err := io.Copy(f, base64Dec); err != nil {
		return filePath, errors.Wrap(err, "error decoding workload kubeconfig")
	}
	return filePath, nil
}
