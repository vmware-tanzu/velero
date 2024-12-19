// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/crash-diagnostics/ssh"
	"github.com/vmware-tanzu/crash-diagnostics/util"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// addDefaultCrashdConf initializes a Starlark Dict with default
// crashd_config configuration data
func addDefaultCrashdConf(thread *starlark.Thread) error {
	args := []starlark.Tuple{
		{starlark.String("gid"), starlark.String(getGid())},
		{starlark.String("uid"), starlark.String(getUid())},
		{starlark.String("workdir"), starlark.String(defaults.workdir)},
	}

	_, err := crashdConfigFn(thread, nil, nil, args)
	if err != nil {
		return err
	}

	return nil
}

// crashdConfigFn is built-in starlark function that saves and returns the kwargs as a struct value.
// Starlark format: crashd_config(workdir=path, default_shell=shellpath, requires=["command0",...,"commandN"])
func crashdConfigFn(thread *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var workdir, gid, uid, defaultShell string
	var useSSHAgent bool
	requires := starlark.NewList([]starlark.Value{})

	if err := starlark.UnpackArgs(
		identifiers.crashdCfg, args, kwargs,
		"workdir?", &workdir,
		"gid?", &gid,
		"uid?", &uid,
		"default_shell?", &defaultShell,
		"requires?", &requires,
		"use_ssh_agent?", &useSSHAgent,
	); err != nil {
		return starlark.None, fmt.Errorf("%s: %s", identifiers.crashdCfg, err)
	}

	// validate
	if len(workdir) == 0 {
		workdir = defaults.workdir
	}

	if len(gid) == 0 {
		gid = getGid()
	}

	if len(uid) == 0 {
		uid = getUid()
	}

	if err := makeCrashdWorkdir(workdir); err != nil {
		return starlark.None, fmt.Errorf("%s: %s", identifiers.crashdCfg, err)
	}

	if useSSHAgent {
		agent, err := ssh.StartAgent()
		if err != nil {
			return starlark.None, errors.Wrap(err, "failed to start ssh agent")
		}

		// sets the ssh_agent variable in the current Starlark thread
		thread.SetLocal(identifiers.sshAgent, agent)
	}

	workdir, err := util.ExpandPath(workdir)
	if err != nil {
		return starlark.None, err
	}

	cfgStruct := starlarkstruct.FromStringDict(starlark.String(identifiers.crashdCfg), starlark.StringDict{
		"workdir":       starlark.String(workdir),
		"gid":           starlark.String(gid),
		"uid":           starlark.String(uid),
		"default_shell": starlark.String(defaultShell),
		"requires":      requires,
	})

	// save values to be used as default
	thread.SetLocal(identifiers.crashdCfg, cfgStruct)

	return cfgStruct, nil
}

func makeCrashdWorkdir(path string) error {
	if _, err := os.Stat(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	logrus.Debugf("creating working directory %s", path)
	if err := os.MkdirAll(path, 0744); err != nil && !os.IsExist(err) {
		return err
	}

	return nil
}
