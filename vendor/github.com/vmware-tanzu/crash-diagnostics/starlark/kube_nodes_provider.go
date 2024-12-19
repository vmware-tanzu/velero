// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/vmware-tanzu/crash-diagnostics/k8s"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// KubeNodesProviderFn is a built-in starlark function that collects compute resources from a k8s cluster
// Starlark format: kube_nodes_provider([kube_config=kube_config(), ssh_config=ssh_config(), names=["foo", "bar], labels=["bar", "baz"]])
func KubeNodesProviderFn(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {

	var names, labels *starlark.List
	var kubeConfig, sshConfig *starlarkstruct.Struct

	if err := starlark.UnpackArgs(
		identifiers.kubeNodesProvider, args, kwargs,
		"names?", &names,
		"labels?", &labels,
		"kube_config?", &kubeConfig,
		"ssh_config?", &sshConfig,
	); err != nil {
		return starlark.None, errors.Wrap(err, "failed to read args")
	}

	ctx, ok := thread.Local(identifiers.scriptCtx).(context.Context)
	if !ok || ctx == nil {
		return starlark.None, fmt.Errorf("script context not found")
	}

	if kubeConfig == nil {
		kubeConfig = thread.Local(identifiers.kubeCfg).(*starlarkstruct.Struct)
	}
	path, err := getKubeConfigPathFromStruct(kubeConfig)
	if err != nil {
		return starlark.None, errors.Wrap(err, "failed to kubeconfig")
	}

	if sshConfig == nil {
		sshConfig = thread.Local(identifiers.sshCfg).(*starlarkstruct.Struct)
	}

	return newKubeNodesProvider(ctx, path, sshConfig, toSlice(names), toSlice(labels))
}

// newKubeNodesProvider returns a struct with k8s cluster node provider info
func newKubeNodesProvider(ctx context.Context, kubeconfig string, sshConfig *starlarkstruct.Struct, names, labels []string) (*starlarkstruct.Struct, error) {

	searchParams := k8s.SearchParams{
		Names:  names,
		Labels: labels,
	}
	nodeAddresses, err := k8s.GetNodeAddresses(ctx, kubeconfig, searchParams.Names, searchParams.Labels)
	if err != nil {
		return nil, errors.Wrapf(err, "could not fetch node addresses")
	}

	// dictionary for node provider struct
	kubeNodesProviderDict := starlark.StringDict{
		"kind":             starlark.String(identifiers.kubeNodesProvider),
		"transport":        starlark.String("ssh"),
		identifiers.sshCfg: sshConfig,
	}

	// add node info to dictionary
	var nodeIps []starlark.Value
	for _, node := range nodeAddresses {
		nodeIps = append(nodeIps, starlark.String(node))
	}
	kubeNodesProviderDict["hosts"] = starlark.NewList(nodeIps)

	return starlarkstruct.FromStringDict(starlark.String(identifiers.kubeNodesProvider), kubeNodesProviderDict), nil
}
