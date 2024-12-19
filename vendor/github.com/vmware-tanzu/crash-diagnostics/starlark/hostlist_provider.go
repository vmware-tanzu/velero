// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// hostListProvider is a built-in starlark function that collects compute resources as a list of host IPs
// Starlark format: host_list_provider(hosts=<host-list> [, ssh_config=ssh_config()])
func hostListProvider(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var hosts *starlark.List
	var sshCfg *starlarkstruct.Struct

	if err := starlark.UnpackArgs(
		identifiers.crashdCfg, args, kwargs,
		"hosts", &hosts,
		"ssh_config?", &sshCfg,
	); err != nil {
		return starlark.None, fmt.Errorf("%s: %s", identifiers.hostListProvider, err)
	}

	if hosts == nil || hosts.Len() == 0 {
		return starlark.None, fmt.Errorf("%s: missing argument: hosts", identifiers.hostListProvider)
	}

	if sshCfg == nil {
		data := thread.Local(identifiers.sshCfg)
		cfg, ok := data.(*starlarkstruct.Struct)
		if !ok {
			return nil, fmt.Errorf("%s: default ssh_config not found", identifiers.hostListProvider)
		}
		sshCfg = cfg
	}

	cfgStruct := starlark.StringDict{
		"kind":             starlark.String(identifiers.hostListProvider),
		"transport":        starlark.String("ssh"),
		"hosts":            hosts,
		identifiers.sshCfg: sshCfg,
	}

	return starlarkstruct.FromStringDict(starlark.String(identifiers.hostListProvider), cfgStruct), nil
}
