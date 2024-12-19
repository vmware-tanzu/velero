// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/vmware-tanzu/crash-diagnostics/k8s"
)

// KubeConfig returns the kubeconfig that needs to be used by the provider.
// The path of the management kubeconfig file gets returned if the workload cluster name is empty
func KubeConfig(mgmtKubeConfigPath, workloadClusterName, workloadClusterNamespace string) (string, error) {
	var err error

	if workloadClusterNamespace == "" {
		workloadClusterNamespace = "default"
	}
	kubeConfigPath := mgmtKubeConfigPath
	if len(workloadClusterName) != 0 {
		kubeConfigPath, err = k8s.FetchWorkloadConfig(workloadClusterName, workloadClusterNamespace, mgmtKubeConfigPath)
		if err != nil {
			err = errors.Wrap(err, fmt.Sprintf("could not fetch kubeconfig for workload cluster %s", workloadClusterName))
		}
	}
	return kubeConfigPath, err
}
