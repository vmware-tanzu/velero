// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/vmware-tanzu/crash-diagnostics/util"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// KubeConfigFn is built-in starlark function that wraps the kwargs into a dictionary value.
// The result is also added to the thread for other built-in to access.
// Starlark: kube_config(path=kubecf/path, [cluster_context=context_name])
func KubeConfigFn(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var path, clusterCtxName string
	var provider *starlarkstruct.Struct

	if err := starlark.UnpackArgs(
		identifiers.kubeCfg, args, kwargs,
		"cluster_context?", &clusterCtxName,
		"path?", &path,
		"capi_provider?", &provider,
	); err != nil {
		return starlark.None, fmt.Errorf("%s: %s", identifiers.kubeCfg, err)
	}

	// check if only one of the two options are present
	if (len(path) == 0 && provider == nil) || (len(path) != 0 && provider != nil) {
		return starlark.None, errors.New("need either path or capi_provider")
	}

	if len(path) == 0 {
		val := provider.Constructor()
		if constructor, ok := val.(starlark.String); ok {
			constStr := constructor.GoString()
			if constStr != identifiers.capvProvider && constStr != identifiers.capaProvider {
				return starlark.None, errors.New("unknown capi provider")
			}
		}

		pathVal, err := provider.Attr("kube_config")
		if err != nil {
			return starlark.None, errors.Wrap(err, "could not find the kubeconfig attribute")
		}
		pathStr, ok := pathVal.(starlark.String)
		if !ok {
			return starlark.None, errors.New("could not fetch kubeconfig")
		}
		path = pathStr.GoString()
	}

	path, err := util.ExpandPath(path)
	if err != nil {
		return starlark.None, err
	}

	structVal := starlarkstruct.FromStringDict(starlark.String(identifiers.kubeCfg), starlark.StringDict{
		"cluster_context": starlark.String(clusterCtxName),
		"path":            starlark.String(path),
	})

	return structVal, nil
}

// addDefaultKubeConf initializes a Starlark Dict with default
// KUBECONFIG configuration data
func addDefaultKubeConf(thread *starlark.Thread) error {
	args := []starlark.Tuple{
		{starlark.String("path"), starlark.String(defaults.kubeconfig)},
	}

	conf, err := KubeConfigFn(thread, nil, nil, args)
	if err != nil {
		return err
	}
	thread.SetLocal(identifiers.kubeCfg, conf)
	return nil
}

func getKubeConfigPathFromStruct(kubeConfigStructVal *starlarkstruct.Struct) (string, error) {
	kvPathVal, err := kubeConfigStructVal.Attr("path")
	if err != nil {
		return "", errors.Wrap(err, "failed to extract kubeconfig path")
	}
	kvPathStrVal, ok := kvPathVal.(starlark.String)
	if !ok {
		return "", errors.New("failed to extract kubeconfig")
	}
	return kvPathStrVal.GoString(), nil
}

// getKubeConfigContextNameFromStruct returns the cluster name from the KubeConfig struct
// provided. If filed cluster_context not provided or unable to convert, it is returned
// as an empty context.
func getKubeConfigContextNameFromStruct(kubeConfigStructVal *starlarkstruct.Struct) string {
	ctxVal, err := kubeConfigStructVal.Attr("cluster_context")
	if err != nil {
		return ""
	}
	ctxName, ok := ctxVal.(starlark.String)
	if !ok {
		return ""
	}
	return ctxName.GoString()
}
