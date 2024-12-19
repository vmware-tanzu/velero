// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/vmware-tanzu/crash-diagnostics/ssh"
)

type commandResult struct {
	resource string
	result   string
	err      error
}

func (r commandResult) toStarlarkStruct() *starlarkstruct.Struct {
	return starlarkstruct.FromStringDict(
		starlark.String("command_result"),
		starlark.StringDict{
			"resource": starlark.String(r.resource),
			"result":   starlark.String(r.result),
			"err": func() starlark.String {
				if r.err != nil {
					return starlark.String(r.err.Error())
				}
				return ""
			}(),
		},
	)
}

// runFunc is a built-in starlark function that runs a provided command.
// It returns the result of the command as struct containing  information
// about the executed command on the provided compute resources.  If resources
// is not provided, runFunc uses the default resources found in the starlark thread.
// Starlark format: run(cmd="command" [,resources=resources])
func runFunc(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var cmdStr string
	var resources *starlark.List
	if err := starlark.UnpackArgs(
		identifiers.crashdCfg, args, kwargs,
		"cmd", &cmdStr,
		"resources?", &resources,
	); err != nil {
		return starlark.None, fmt.Errorf("%s: %s", identifiers.run, err)
	}

	if resources == nil {
		res := thread.Local(identifiers.resources)
		if res == nil {
			return starlark.None, fmt.Errorf("%s: default resources not found", identifiers.run)
		}
		resList, ok := res.(*starlark.List)
		if !ok {
			return starlark.None, fmt.Errorf("%s: unexpected resources type", identifiers.run)
		}
		resources = resList
	}

	var agent ssh.Agent
	var ok bool
	if agentVal := thread.Local(identifiers.sshAgent); agentVal != nil {
		agent, ok = agentVal.(ssh.Agent)
		if !ok {
			return starlark.None, errors.New("unable to fetch ssh-agent")
		}
	}

	results, err := execRun(cmdStr, agent, resources)
	if err != nil {
		return starlark.None, err
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

func execRun(cmdStr string, agent ssh.Agent, resources *starlark.List) ([]commandResult, error) {
	if resources == nil {
		return nil, fmt.Errorf("%s: missing resources", identifiers.run)
	}

	logrus.Debugf("%s: executing command on %d resources", identifiers.run, resources.Len())
	var results []commandResult
	for i := 0; i < resources.Len(); i++ {
		val := resources.Index(i)
		res, ok := val.(*starlarkstruct.Struct)
		if !ok {
			return nil, fmt.Errorf("%s: unexpected resource type", identifiers.run)
		}

		val, err := res.Attr("kind")
		if err != nil {
			return nil, fmt.Errorf("%s: resource.kind: %s", identifiers.run, err)
		}
		kind := val.(starlark.String)

		val, err = res.Attr("transport")
		if err != nil {
			return nil, fmt.Errorf("%s: resource.transport: %s", identifiers.run, err)
		}
		transport := val.(starlark.String)

		switch {
		case string(kind) == identifiers.hostResource && string(transport) == "ssh":
			result, err := execRunSSH(cmdStr, agent, res)
			if err != nil {
				logrus.Error(err)
				continue
			}
			results = append(results, result)
		default:
			logrus.Errorf("%s: unsupported or invalid resource kind: %s", identifiers.run, kind)
			continue
		}
	}

	return results, nil
}

// execRunSSH executes `run` command for a Host Resource using SSH
func execRunSSH(cmdStr string, agent ssh.Agent, res *starlarkstruct.Struct) (commandResult, error) {
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

	// add host
	hVal, err := res.Attr("host")
	if err != nil {
		return commandResult{}, fmt.Errorf("%s: resource.host: %s", identifiers.run, err)
	}
	host, ok := hVal.(starlark.String)
	if !ok {
		return commandResult{}, fmt.Errorf("%s: resource.host has unexpected type", identifiers.run)
	}
	args.Host = string(host)

	logrus.Debugf("%s: executing command on %s using ssh: [%s]", identifiers.run, args.Host, cmdStr)
	cmdResult, err := ssh.Run(args, agent, cmdStr)
	return commandResult{resource: args.Host, result: cmdResult, err: err}, nil

}

func getSSHArgsFromCfg(sshCfg *starlarkstruct.Struct) (ssh.SSHArgs, error) {
	val, err := sshCfg.Attr(identifiers.username)
	if err != nil {
		return ssh.SSHArgs{}, fmt.Errorf("%s: ssh_config.username: %s", identifiers.run, err)
	}
	user, ok := val.(starlark.String)
	if !ok || len(user) == 0 {
		return ssh.SSHArgs{}, fmt.Errorf("%s: ssh_config.username not found", identifiers.run)
	}

	port := defaults.sshPort
	if val, err = sshCfg.Attr(identifiers.port); err == nil {
		if prt, ok := val.(starlark.String); ok && len(port) > 0 {
			port = string(prt)
		}
	}

	maxRetries := defaults.connRetries
	if val, err := sshCfg.Attr(identifiers.maxRetries); err == nil {
		if retries, ok := val.(starlark.Int); ok {
			maxRetries = int(retries.BigInt().Int64())
		}
	}

	// both jump user/host must be provided, else ignore
	var jumpProxy *ssh.ProxyJumpArgs
	uval, uerr := sshCfg.Attr(identifiers.jumpUser)
	hval, herr := sshCfg.Attr(identifiers.jumpHost)
	if uerr == nil && herr == nil {
		juser := string(uval.(starlark.String))
		jhost := string(hval.(starlark.String))

		if len(juser) > 0 && len(jhost) > 0 {
			jumpProxy = &ssh.ProxyJumpArgs{
				User: juser,
				Host: jhost,
			}
		}
	}

	var privateKeyPath string
	if pkPathVal, err := sshCfg.Attr("private_key_path"); err == nil {
		pkPath := pkPathVal.(starlark.String)
		privateKeyPath = pkPath.GoString()
	}

	args := ssh.SSHArgs{
		User:           string(user),
		Port:           port,
		MaxRetries:     maxRetries,
		ProxyJump:      jumpProxy,
		PrivateKeyPath: privateKeyPath,
	}
	return args, nil
}
