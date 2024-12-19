// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/crash-diagnostics/ssh"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// captureFunc is a built-in starlark function that runs a provided command and
// captures the result of the command in a specified file stored in workdir.
// If resources and workdir are not provided, captureFunc uses defaults from starlark thread generated
// by previous calls to resources() and crashd_config().
// Starlark format: capture(command-string, cmd="command" [,resources=resources][,workdir=path][,file_name=name][,desc=description])
func captureFunc(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var cmdStr, workdir, fileName, desc string
	var resources *starlark.List

	if err := starlark.UnpackArgs(
		identifiers.capture, args, kwargs,
		"cmd", &cmdStr,
		"resources?", &resources,
		"workdir?", &workdir,
		"file_name?", &fileName,
		"desc?", &desc,
	); err != nil {
		return starlark.None, fmt.Errorf("%s: %s", identifiers.capture, err)
	}

	if len(cmdStr) == 0 {
		return starlark.None, fmt.Errorf("%s: missing command string", identifiers.capture)
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

	results, err := execCapture(cmdStr, workdir, fileName, desc, agent, resources)
	if err != nil {
		return starlark.None, fmt.Errorf("%s: %s", identifiers.capture, err)
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

func execCapture(cmdStr, rootPath, fileName, desc string, agent ssh.Agent, resources *starlark.List) ([]commandResult, error) {
	if resources == nil {
		return nil, fmt.Errorf("%s: missing resources", identifiers.capture)
	}

	logrus.Debugf("%s: executing command on %d resources", identifiers.capture, resources.Len())
	var results []commandResult
	for i := 0; i < resources.Len(); i++ {
		val := resources.Index(i)
		res, ok := val.(*starlarkstruct.Struct)
		if !ok {
			return nil, fmt.Errorf("%s: unexpected resource type", identifiers.capture)
		}

		val, err := res.Attr("kind")
		if err != nil {
			return nil, fmt.Errorf("%s: resource.kind: %s", identifiers.capture, err)
		}
		kind := val.(starlark.String)

		val, err = res.Attr("transport")
		if err != nil {
			return nil, fmt.Errorf("%s: resource.transport: %s", identifiers.capture, err)
		}
		transport := val.(starlark.String)

		val, err = res.Attr("host")
		if err != nil {
			return nil, fmt.Errorf("%s: resource.host: %s", identifiers.capture, err)
		}
		host := string(val.(starlark.String))
		rootDir := filepath.Join(rootPath, sanitizeStr(host))

		switch {
		case string(kind) == identifiers.hostResource && string(transport) == "ssh":
			result, err := execCaptureSSH(host, cmdStr, rootDir, fileName, desc, agent, res)
			if err != nil {
				logrus.Errorf("%s failed: cmd=[%s]: %s", identifiers.capture, cmdStr, err)
			}
			results = append(results, result)
		default:
			logrus.Errorf("%s: unsupported or invalid resource kind: %s", identifiers.capture, kind)
			continue
		}
	}

	return results, nil
}

func execCaptureSSH(host, cmdStr, rootDir, fileName, desc string, agent ssh.Agent, res *starlarkstruct.Struct) (commandResult, error) {
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
	logrus.Debugf("%s: created capture dir: %s", identifiers.capture, rootDir)

	if len(fileName) == 0 {
		fileName = fmt.Sprintf("%s.txt", sanitizeStr(cmdStr))
	}
	filePath := filepath.Join(rootDir, fileName)

	logrus.Debugf("%s: capturing output of [cmd=%s] => [%s] from %s using ssh", identifiers.capture, cmdStr, filePath, args.Host)

	reader, err := ssh.RunRead(args, agent, cmdStr)
	if err != nil {
		logrus.Errorf("%s failed: %s", identifiers.capture, err)
		if err := captureOutput(strings.NewReader(err.Error()), filePath, fmt.Sprintf("%s: failed", cmdStr), false); err != nil {
			logrus.Errorf("%s output failed: %s", identifiers.capture, err)
			return commandResult{resource: args.Host, result: filePath, err: err}, err
		}
	}

	if err := captureOutput(reader, filePath, desc, false); err != nil {
		logrus.Errorf("%s output failed: %s", identifiers.capture, err)
		return commandResult{resource: args.Host, result: filePath, err: err}, err
	}

	return commandResult{resource: args.Host, result: filePath, err: err}, nil
}

func captureOutput(source io.Reader, filePath, desc string, append bool) error {
	if source == nil {
		return fmt.Errorf("source reader is nill")
	}

	flag := os.O_CREATE | os.O_WRONLY
	if append {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}

	file, err := os.OpenFile(filePath, flag, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	if len(desc) > 0 {
		if _, err := file.WriteString(fmt.Sprintf("%s\n", desc)); err != nil {
			return err
		}
	}

	if _, err := io.Copy(file, source); err != nil {
		return err
	}

	return nil
}
