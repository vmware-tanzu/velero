// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"context"

	"github.com/pkg/errors"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func GetNodeAddresses(ctx context.Context, kubeconfigPath string, names, labels []string) ([]string, error) {
	client, err := New(kubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "could not initialize search client")
	}

	nodes, err := getNodes(ctx, client, names, labels)
	if err != nil {
		return nil, errors.Wrapf(err, "could not fetch nodes")
	}

	var nodeIps []string
	for _, node := range nodes {
		nodeIps = append(nodeIps, getNodeInternalIP(node))
	}
	return nodeIps, nil
}

func getNodes(ctx context.Context, k8sc *Client, names, labels []string) ([]*coreV1.Node, error) {
	nodeResults, err := k8sc.Search(ctx, SearchParams{
		Groups: []string{"core"},
		Kinds:  []string{"nodes"},
		Names:  names,
		Labels: labels,
	})
	if err != nil {
		return nil, err
	}

	// collate
	var nodes []*coreV1.Node
	for _, result := range nodeResults {
		for _, item := range result.List.Items {
			node := new(coreV1.Node)
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &node); err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func getNodeInternalIP(node *coreV1.Node) (ipAddr string) {
	for _, addr := range node.Status.Addresses {
		if addr.Type == "InternalIP" {
			ipAddr = addr.Address
			return
		}
	}
	return
}
