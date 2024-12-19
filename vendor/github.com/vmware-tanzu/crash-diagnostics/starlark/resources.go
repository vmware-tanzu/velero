// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// resourcesFunc is a built-in starlark function that prepares returns compute list of resources.
// Starlark format: resources(provider=<provider-function>)
func resourcesFunc(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var hosts *starlark.List
	var provider *starlarkstruct.Struct
	if err := starlark.UnpackArgs(
		identifiers.crashdCfg, args, kwargs,
		"hosts?", &hosts,
		"provider?", &provider,
	); err != nil {
		return starlark.None, fmt.Errorf("%s: %s", identifiers.hostListProvider, err)
	}

	if hosts == nil && provider == nil {
		return starlark.None, fmt.Errorf("%s: hosts or provider argument required", identifiers.resources)
	}

	if hosts != nil && provider != nil {
		return starlark.None, fmt.Errorf("%s: specify hosts or provider argument", identifiers.resources)
	}

	if hosts != nil {
		prov, err := hostListProvider(thread, nil, nil, []starlark.Tuple{{starlark.String("hosts"), hosts}})
		if err != nil {
			return starlark.None, err
		}
		provider = prov.(*starlarkstruct.Struct)
	}

	// enumerate resources from provider
	resources, err := enum(provider)
	if err != nil {
		return starlark.None, err
	}

	return resources, nil
}

// enum returns a list of structs containing the fully enumerated compute resource
// info needed to execute commands.
func enum(provider *starlarkstruct.Struct) (*starlark.List, error) {
	if provider == nil {
		return nil, fmt.Errorf("missing provider")
	}

	var resources []starlark.Value

	kindVal, err := provider.Attr("kind")
	if err != nil {
		return nil, fmt.Errorf("provider missing field kind")
	}

	kind := trimQuotes(kindVal.String())

	switch kind {
	case identifiers.hostListProvider, identifiers.kubeNodesProvider, identifiers.capvProvider, identifiers.capaProvider:
		hosts, err := provider.Attr("hosts")
		if err != nil {
			return nil, fmt.Errorf("hosts not found in %s", identifiers.hostListProvider)
		}

		hostList, ok := hosts.(*starlark.List)
		if !ok {
			return nil, fmt.Errorf("%s: unexpected type for hosts: %T", identifiers.hostListProvider, hosts)
		}

		transport, err := provider.Attr("transport")
		if err != nil {
			return nil, fmt.Errorf("transport not found in %s", identifiers.hostListProvider)
		}

		sshCfg, err := provider.Attr(identifiers.sshCfg)
		if err != nil {
			return nil, fmt.Errorf("ssh_config not found in %s", identifiers.hostListProvider)
		}

		for i := 0; i < hostList.Len(); i++ {
			dict := starlark.StringDict{
				"kind":       starlark.String(identifiers.hostResource),
				"provider":   starlark.String(identifiers.hostListProvider),
				"host":       hostList.Index(i),
				"transport":  transport,
				"ssh_config": sshCfg,
			}
			resources = append(resources, starlarkstruct.FromStringDict(starlark.String(identifiers.hostResource), dict))
		}
	}

	return starlark.NewList(resources), nil
}
