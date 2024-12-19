// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/crash-diagnostics/k8s"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// KubeCaptureFn is the Starlark built-in for the fetching kubernetes objects
// and returns the result as a Starlark value containing the file path and error message, if any
// Starlark format: kube_capture(what="logs" [, groups="core", namespaces=["default"], kube_config=kube_config()])
func KubeCaptureFn(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {

	var groups, categories, kinds, namespaces, versions, names, labels, containers *starlark.List
	var kubeConfig *starlarkstruct.Struct
	var what string

	if err := starlark.UnpackArgs(
		identifiers.kubeCapture, args, kwargs,
		"what", &what,
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
		return starlark.None, errors.Wrap(err, "failed to kubeconfig")
	}
	clusterCtxName := getKubeConfigContextNameFromStruct(kubeConfig)

	client, err := k8s.New(path, clusterCtxName)
	if err != nil {
		return starlark.None, errors.Wrap(err, "could not initialize search client")
	}

	data := thread.Local(identifiers.crashdCfg)
	cfg, _ := data.(*starlarkstruct.Struct)
	workDirVal, _ := cfg.Attr("workdir")
	resultDir, err := write(ctx, trimQuotes(workDirVal.String()), what, client, k8s.SearchParams{
		Groups:     toSlice(groups),
		Categories: toSlice(categories),
		Kinds:      toSlice(kinds),
		Namespaces: toSlice(namespaces),
		Versions:   toSlice(versions),
		Names:      toSlice(names),
		Labels:     toSlice(labels),
		Containers: toSlice(containers),
	})

	return starlarkstruct.FromStringDict(
		starlark.String(identifiers.kubeCapture),
		starlark.StringDict{
			"file": starlark.String(resultDir),
			"error": func() starlark.String {
				if err != nil {
					return starlark.String(err.Error())
				}
				return ""
			}(),
		}), nil
}

func write(ctx context.Context, workdir, what string, client *k8s.Client, params k8s.SearchParams) (string, error) {

	logrus.Debugf("kube_capture(what=%s)", what)
	switch what {
	case "logs":
		params.Groups = []string{"core"}
		params.Kinds = []string{"pods"}
		params.Versions = []string{}
	case "objects", "all", "*":
	default:
		return "", errors.Errorf("don't know how to get: %s", what)
	}

	searchResults, err := client.Search(ctx, params)
	if err != nil {
		return "", err
	}

	resultWriter, err := k8s.NewResultWriter(workdir, what, client.CoreRest)
	if err != nil {
		return "", errors.Wrap(err, "failed to initialize writer")
	}
	err = resultWriter.Write(ctx, searchResults)
	if err != nil {
		return "", errors.Wrap(err, "failed to write search results")
	}
	return resultWriter.GetResultDir(), nil
}
