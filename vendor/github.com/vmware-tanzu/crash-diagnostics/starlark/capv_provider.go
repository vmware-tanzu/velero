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

// CapvProviderFn is a built-in starlark function that collects compute resources from a k8s cluster
// Starlark format: capv_provider(kube_config=kube_config(), ssh_config=ssh_config()[workload_cluster=<name>, namespace=<namespace>, nodes=["foo", "bar], labels=["bar", "baz"]])
func CapvProviderFn(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {

	var (
		workloadCluster, namespace string
		names, labels              *starlark.List
		sshConfig, mgmtKubeConfig  *starlarkstruct.Struct
	)

	err := starlark.UnpackArgs("capv_provider", args, kwargs,
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
		return starlark.None, errors.New("capv_provider requires the name of the management cluster, the ssh configuration and the management cluster kubeconfig")
	}

	if mgmtKubeConfig == nil {
		mgmtKubeConfig = thread.Local(identifiers.kubeCfg).(*starlarkstruct.Struct)
	}
	mgmtKubeConfigPath, err := getKubeConfigPathFromStruct(mgmtKubeConfig)
	if err != nil {
		return starlark.None, errors.Wrap(err, "failed to extract management kubeconfig")
	}

	providerConfigPath, err := provider.KubeConfig(mgmtKubeConfigPath, workloadCluster, namespace)
	if err != nil {
		return starlark.None, err
	}

	nodeAddresses, err := k8s.GetNodeAddresses(ctx, providerConfigPath, toSlice(names), toSlice(labels))
	if err != nil {
		return starlark.None, errors.Wrap(err, "could not fetch host addresses")
	}

	// dictionary for capv provider struct
	capvProviderDict := starlark.StringDict{
		"kind":        starlark.String(identifiers.capvProvider),
		"transport":   starlark.String("ssh"),
		"kube_config": starlark.String(providerConfigPath),
	}

	// add node info to dictionary
	var nodeIps []starlark.Value
	for _, node := range nodeAddresses {
		nodeIps = append(nodeIps, starlark.String(node))
	}
	capvProviderDict["hosts"] = starlark.NewList(nodeIps)

	// add ssh info to dictionary
	if _, ok := capvProviderDict[identifiers.sshCfg]; !ok {
		capvProviderDict[identifiers.sshCfg] = sshConfig
	}

	return starlarkstruct.FromStringDict(starlark.String(identifiers.capvProvider), capvProviderDict), nil
}

// TODO: Needs to be moved to a single package
func toSlice(list *starlark.List) []string {
	var elems []string
	if list != nil {
		for i := 0; i < list.Len(); i++ {
			if val, ok := list.Index(i).(starlark.String); ok {
				elems = append(elems, string(val))
			}
		}
	}
	return elems
}
