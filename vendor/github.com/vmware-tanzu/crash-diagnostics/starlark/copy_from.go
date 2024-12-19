// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/crash-diagnostics/ssh"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// copyFromFunc is a built-in starlark function that copies file resources from
// specified compute resources and saves them on the local machine
// in subdirectory under workdir.
//
// If resources and workdir are not provided, copyFromFunc uses defaults from starlark thread generated
// by previous calls to resources(), ssh_config, and crashd_config().
//
// Starlark format: copy_from([<path>] [,path=<list>, resources=resources, workdir=path])
func copyFromFunc(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var sourcePath, workdir string
	var resources *starlark.List

	if err := starlark.UnpackArgs(
		identifiers.capture, args, kwargs,
		"path", &sourcePath,
		"resources?", &resources,
		"workdir?", &workdir,
	); err != nil {
		return starlark.None, fmt.Errorf("%s: %s", identifiers.capture, err)
	}

	if len(sourcePath) == 0 {
		return starlark.None, fmt.Errorf("%s: path arg not set", identifiers.copyFrom)
	}

	if len(workdir) == 0 {
		if dir, err := getWorkdirFromThread(thread); err == nil {
			workdir = dir
		}
	}
	if len(workdir) == 0 {
		workdir = defaults.workdir
	}

	if resources == nil {
		res, err := getResourcesFromThread(thread)
		if err != nil {
			return starlark.None, fmt.Errorf("%s: %s", identifiers.copyFrom, err)
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

	results, err := execCopyFrom(workdir, sourcePath, agent, resources)
	if err != nil {
		return starlark.None, fmt.Errorf("%s: %s", identifiers.copyFrom, err)
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

func execCopyFrom(rootPath string, path string, agent ssh.Agent, resources *starlark.List) ([]commandResult, error) {
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
		rootDir := filepath.Join(rootPath, sanitizeStr(host))

		switch {
		case string(kind) == identifiers.hostResource && string(transport) == "ssh":
			result, err := execSCPCopyFrom(host, rootDir, path, agent, res)
			if err != nil {
				logrus.Errorf("%s: failed to copyFrom %s: %s", identifiers.copyFrom, path, err)
			}
			results = append(results, result)
		default:
			logrus.Errorf("%s: unsupported or invalid resource kind: %s", identifiers.copyFrom, kind)
			continue
		}
	}

	return results, nil
}

func execSCPCopyFrom(host, rootDir, path string, agent ssh.Agent, res *starlarkstruct.Struct) (commandResult, error) {
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

	// create dir for the host
	if err := os.MkdirAll(rootDir, 0744); err != nil && !os.IsExist(err) {
		return commandResult{}, err
	}

	err = ssh.CopyFrom(args, agent, rootDir, path)
	return commandResult{resource: args.Host, result: filepath.Join(rootDir, path), err: err}, err
}
