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

// KubeGetFn is a starlark built-in for the fetching kubernetes objects
func KubeGetFn(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	objects := starlark.NewList([]starlark.Value{})
	var groups, categories, kinds, namespaces, versions, names, labels, containers *starlark.List
	var kubeConfig *starlarkstruct.Struct

	if err := starlark.UnpackArgs(
		identifiers.kubeGet, args, kwargs,
		"groups?", &groups,
		"categories?", &categories,
		"kinds?", &kinds,
		"namespaces?", &namespaces,
		"versions?", &versions,
		"names?", &names,
		"labels?", &labels,
		"containers?", &containers,
		"kube_config?", &kubeConfig,
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
		return starlark.None, errors.Wrap(err, "failed to get kubeconfig")
	}
	clusterCtxName := getKubeConfigContextNameFromStruct(kubeConfig)

	client, err := k8s.New(path, clusterCtxName)
	if err != nil {
		return starlark.None, errors.Wrap(err, "could not initialize search client")
	}

	searchParams := k8s.SearchParams{
		Groups:     toSlice(groups),
		Categories: toSlice(categories),
		Kinds:      toSlice(kinds),
		Namespaces: toSlice(namespaces),
		Versions:   toSlice(versions),
		Names:      toSlice(names),
		Labels:     toSlice(labels),
		Containers: toSlice(containers),
	}
	searchResults, searchErr := client.Search(ctx, searchParams)
	if searchErr == nil {
		for _, searchResult := range searchResults {
			srValue := searchResult.ToStarlarkValue()
			if err := objects.Append(srValue); err != nil {
				searchErr = errors.Wrap(err, "could not collect kube_get() results")
				break
			}
		}
	}

	return starlarkstruct.FromStringDict(
		starlark.String(identifiers.kubeGet),
		starlark.StringDict{
			"objs": objects,
			"error": func() starlark.String {
				if searchErr != nil {
					return starlark.String(searchErr.Error())
				}
				return ""
			}(),
		}), nil
}
