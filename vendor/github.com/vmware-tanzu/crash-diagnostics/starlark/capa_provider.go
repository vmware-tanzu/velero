// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/vmware-tanzu/crash-diagnostics/k8s"
	"github.com/vmware-tanzu/crash-diagnostics/provider"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// CapaProviderFn is a built-in starlark function that collects compute resources from a k8s cluster
// Starlark format: capa_provider(kube_config=kube_config(), ssh_config=ssh_config()[workload_cluster=<name>, namespace=<namespace>, nodes=["foo", "bar], labels=["bar", "baz"]])
func CapaProviderFn(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {

	var (
		workloadCluster, namespace string
		names, labels              *starlark.List
		sshConfig, mgmtKubeConfig  *starlarkstruct.Struct
	)

	err := starlark.UnpackArgs("capa_provider", args, kwargs,
		"ssh_config", &sshConfig,
		"mgmt_kube_config", &mgmtKubeConfig,
		"workload_cluster?", &workloadCluster,
		"namespace?", &namespace,
		"labels?", &labels,
		"nodes?", &names)
	if err != nil {
		return starlark.None, errors.Wrap(err, "failed to unpack input arguments")
	}

	ctx, ok := thread.Local(identifiers.scriptCtx).(context.Context)
	if !ok || ctx == nil {
		return starlark.None, fmt.Errorf("script context not found")
	}

	if sshConfig == nil || mgmtKubeConfig == nil {
		return starlark.None, errors.New("capa_provider requires the name of the management cluster, the ssh configuration and the management cluster kubeconfig")
	}

	if mgmtKubeConfig == nil {
		mgmtKubeConfig = thread.Local(identifiers.kubeCfg).(*starlarkstruct.Struct)
	}
	mgmtKubeConfigPath, err := getKubeConfigPathFromStruct(mgmtKubeConfig)
	if err != nil {
		return starlark.None, errors.Wrap(err, "failed to extract management kubeconfig")
	}

	// if workload cluster is not supplied, then the resources for the management cluster
	// should be enumerated
	clusterName := workloadCluster
	if clusterName == "" {
		config, err := k8s.LoadKubeCfg(mgmtKubeConfigPath)
		if err != nil {
			return starlark.None, errors.Wrap(err, "failed to load kube config")
		}
		clusterName, err = config.GetClusterName()
		if err != nil {
			return starlark.None, errors.Wrap(err, "cannot find cluster with name "+workloadCluster)
		}
	}

	bastionIpAddr, err := k8s.FetchBastionIpAddress(clusterName, namespace, mgmtKubeConfigPath)
	if err != nil {
		return starlark.None, errors.Wrap(err, "could not fetch jump host addresses")
	}

	providerConfigPath, err := provider.KubeConfig(mgmtKubeConfigPath, clusterName, namespace)
	if err != nil {
		return starlark.None, err
	}

	nodeAddresses, err := k8s.GetNodeAddresses(ctx, providerConfigPath, toSlice(names), toSlice(labels))
	if err != nil {
		return starlark.None, errors.Wrap(err, "could not fetch host addresses")
	}

	// dictionary for capa provider struct
	capaProviderDict := starlark.StringDict{
		"kind":        starlark.String(identifiers.capaProvider),
		"transport":   starlark.String("ssh"),
		"kube_config": starlark.String(providerConfigPath),
	}

	// add node info to dictionary
	var nodeIps []starlark.Value
	for _, node := range nodeAddresses {
		nodeIps = append(nodeIps, starlark.String(node))
	}
	capaProviderDict["hosts"] = starlark.NewList(nodeIps)

	sshConfigDict := starlark.StringDict{}
	sshConfig.ToStringDict(sshConfigDict)

	// modify ssh config jump credentials, if not specified
	if _, err := sshConfig.Attr("jump_host"); err != nil {
		sshConfigDict["jump_host"] = starlark.String(bastionIpAddr)
	}
	if _, err := sshConfig.Attr("jump_user"); err != nil {
		sshConfigDict["jump_user"] = starlark.String("ubuntu")
	}
	capaProviderDict[identifiers.sshCfg] = starlarkstruct.FromStringDict(starlark.String(identifiers.sshCfg), sshConfigDict)

	return starlarkstruct.FromStringDict(starlark.String(identifiers.capaProvider), capaProviderDict), nil
}
