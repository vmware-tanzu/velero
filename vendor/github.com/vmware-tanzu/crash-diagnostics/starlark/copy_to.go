// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/vmware-tanzu/crash-diagnostics/ssh"
)

// copyToFunc is a built-in starlark function that copies file resources from
// the local machine to a specified location on remote compute resources.
//
// If only one argument is provided, it is assumed to be the <source path> of file to copy.
// If resources are not provded, copy_to will search the starlark context for one.
//
// Starlark format: copy_to([<source_path>] [,source_path=<file path>, target_path=<file path>, resources=resources])

func copyToFunc(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var sourcePath, targetPath string
	var resources *starlark.List

	if err := starlark.UnpackArgs(
		identifiers.copyTo, args, kwargs,
		"source_path", &sourcePath,
		"target_path?", &targetPath,
		"resources?", &resources,
	); err != nil {
		return starlark.None, fmt.Errorf("%s: %s", identifiers.copyTo, err)
	}

	if len(sourcePath) == 0 {
		return starlark.None, fmt.Errorf("%s: path arg not set", identifiers.copyTo)
	}
	if len(targetPath) == 0 {
		targetPath = sourcePath
	}

	if resources == nil {
		res, err := getResourcesFromThread(thread)
		if err != nil {
			return starlark.None, fmt.Errorf("%s: %s", identifiers.copyTo, err)
		}
		resources = res
	}

	var agent ssh.Agent
	var ok bool
	if agentVal := thread.Local(identifiers.sshAgent); agentVal != nil {
		agent, ok = agentVal.(ssh.Agent)
		if !ok {
			return starlark.None, errors.New("unable to fetch ssh-agent")
		}
	}

	results, err := execCopyTo(sourcePath, targetPath, agent, resources)
	if err != nil {
		return starlark.None, fmt.Errorf("%s: %s", identifiers.copyTo, err)
	}

	// build list of struct as result
	var resultList []starlark.Value
	for _, result := range results {
		if len(results) == 1 {
			return result.toStarlarkStruct(), nil
		}
		resultList = append(resultList, result.toStarlarkStruct())
	}

	return starlark.NewList(resultList), nil
}

func execCopyTo(sourcePath, targetPath string, agent ssh.Agent, resources *starlark.List) ([]commandResult, error) {
	if resources == nil {
		return nil, fmt.Errorf("%s: missing resources", identifiers.copyFrom)
	}

	var results []commandResult
	for i := 0; i < resources.Len(); i++ {
		val := resources.Index(i)
		res, ok := val.(*starlarkstruct.Struct)
		if !ok {
			return nil, fmt.Errorf("%s: unexpected resource type", identifiers.copyFrom)
		}

		val, err := res.Attr("kind")
		if err != nil {
			return nil, fmt.Errorf("%s: resource.kind: %s", identifiers.copyFrom, err)
		}
		kind := val.(starlark.String)

		val, err = res.Attr("transport")
		if err != nil {
			return nil, fmt.Errorf("%s: resource.transport: %s", identifiers.copyFrom, err)
		}
		transport := val.(starlark.String)

		val, err = res.Attr("host")
		if err != nil {
			return nil, fmt.Errorf("%s: resource.host: %s", identifiers.copyFrom, err)
		}
		host := string(val.(starlark.String))

		switch {
		case string(kind) == identifiers.hostResource && string(transport) == "ssh":
			result, err := execSCPCopyTo(host, sourcePath, targetPath, agent, res)
			if err != nil {
				logrus.Errorf("%s: failed to copy to : %s: %s", identifiers.copyTo, sourcePath, err)
			}
			results = append(results, result)
		default:
			logrus.Errorf("%s: unsupported or invalid resource kind: %s", identifiers.copyFrom, kind)
			continue
		}
	}

	return results, nil
}

func execSCPCopyTo(host, sourcePath, targetPath string, agent ssh.Agent, res *starlarkstruct.Struct) (commandResult, error) {
	sshCfg := starlarkstruct.FromKeywords(starlarkstruct.Default, makeDefaultSSHConfig())
	if val, err := res.Attr(identifiers.sshCfg); err == nil {
		if cfg, ok := val.(*starlarkstruct.Struct); ok {
			sshCfg = cfg
		}
	}

	args, err := getSSHArgsFromCfg(sshCfg)
	if err != nil {
		return commandResult{}, err
	}
	args.Host = host
	err = ssh.CopyTo(args, agent, sourcePath, targetPath)
	return commandResult{resource: args.Host, result: targetPath, err: err}, err
}
