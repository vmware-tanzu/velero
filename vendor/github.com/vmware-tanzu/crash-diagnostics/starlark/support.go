// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

var (
	strSanitization = regexp.MustCompile(`[^a-zA-Z0-9]`)

	identifiers = struct {
		scriptCtx string

		crashdCfg string
		kubeCfg   string

		sshCfg         string
		port           string
		username       string
		privateKeyPath string
		maxRetries     string
		jumpUser       string
		jumpHost       string

		hostListProvider string
		hostResource     string
		resources        string
		run              string
		runLocal         string
		progAvailLocal   string
		capture          string
		captureLocal     string
		copyFrom         string
		copyTo           string
		archive          string
		os               string
		setDefaults      string
		log              string

		kubeCapture       string
		kubeGet           string
		kubeNodesProvider string
		capvProvider      string
		capaProvider      string

		sshAgent string
	}{
		scriptCtx: "script_context",

		crashdCfg: "crashd_config",
		kubeCfg:   "kube_config",
		sshCfg:    "ssh_config",

		port:           "port",
		username:       "username",
		privateKeyPath: "private_key_path",
		maxRetries:     "max_retries",
		jumpUser:       "jump_user",
		jumpHost:       "jump_host",

		hostListProvider: "host_list_provider",
		hostResource:     "host_resource",
		resources:        "resources",
		run:              "run",
		runLocal:         "run_local",
		progAvailLocal:   "prog_avail_local",
		capture:          "capture",
		captureLocal:     "capture_local",
		copyFrom:         "copy_from",
		copyTo:           "copy_to",
		archive:          "archive",
		os:               "os",
		setDefaults:      "set_defaults",
		log:              "log",

		kubeCapture:       "kube_capture",
		kubeGet:           "kube_get",
		kubeNodesProvider: "kube_nodes_provider",
		capvProvider:      "capv_provider",
		capaProvider:      "capa_provider",

		sshAgent: "crashd_ssh_agent",
	}

	defaults = struct {
		crashdir    string
		workdir     string
		kubeconfig  string
		sshPort     string
		pkPath      string
		outPath     string
		connRetries int
		connTimeout int // seconds
	}{
		crashdir: filepath.Join(os.Getenv("HOME"), ".crashd"),
		workdir:  "/tmp/crashd",
		kubeconfig: func() string {
			kubecfg := os.Getenv("KUBECONFIG")
			if kubecfg == "" {
				kubecfg = filepath.Join(os.Getenv("HOME"), ".kube", "config")
			}
			return kubecfg
		}(),
		sshPort: "22",
		pkPath: func() string {
			return filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa")
		}(),
		connRetries: 30,
		connTimeout: 30,
	}
)

func getWorkdirFromThread(thread *starlark.Thread) (string, error) {
	val := thread.Local(identifiers.crashdCfg)
	if val == nil {
		return "", fmt.Errorf("%s not found in threard", identifiers.crashdCfg)
	}
	var result string
	if valStruct, ok := val.(*starlarkstruct.Struct); ok {
		if valStr, err := valStruct.Attr("workdir"); err == nil {
			if str, ok := valStr.(starlark.String); ok {
				result = string(str)
			}
		}
	}

	if len(result) == 0 {
		result = defaults.workdir
	}
	return result, nil
}

func getResourcesFromThread(thread *starlark.Thread) (*starlark.List, error) {
	var resources *starlark.List
	res := thread.Local(identifiers.resources)
	if res == nil {
		return nil, fmt.Errorf("%s not found in thread", identifiers.resources)
	}
	if resList, ok := res.(*starlark.List); ok {
		resources = resList
	}
	if resources == nil {
		return nil, fmt.Errorf("%s missing or invalid", identifiers.resources)
	}
	return resources, nil
}

func trimQuotes(val string) string {
	unquoted, err := strconv.Unquote(val)
	if err != nil {
		return val
	}
	return unquoted
}

func getUsername() string {
	usr, err := user.Current()
	if err != nil {
		return ""
	}
	return usr.Username
}

func getUid() string {
	usr, err := user.Current()
	if err != nil {
		return ""
	}
	return usr.Uid
}

func getGid() string {
	usr, err := user.Current()
	if err != nil {
		return ""
	}
	return usr.Gid
}

func sanitizeStr(str string) string {
	return strSanitization.ReplaceAllString(str, "_")
}
