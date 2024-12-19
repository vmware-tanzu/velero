// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package starlark

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
	"github.com/vmware-tanzu/crash-diagnostics/ssh"
	"go.starlark.net/starlark"
)

type Executor struct {
	thread  *starlark.Thread
	predecs starlark.StringDict
	result  starlark.StringDict
}

func New() *Executor {
	return &Executor{
		thread:  &starlark.Thread{Name: "crashd"},
		predecs: newPredeclareds(),
	}
}

// AddPredeclared predeclared
func (e *Executor) AddPredeclared(name string, value starlark.Value) {
	if e.predecs != nil {
		e.predecs[name] = value
	}
}

// Preload loads parse and load code modules to be used in other scripts.
// This call should take place prior to calling the Exec call for main script execution.
// The result of the loaded module are added to the global predeclared
// values used in Exec call.
func (e *Executor) Preload(name string, source io.Reader) error {
	result, err := starlark.ExecFile(e.thread, name, source, e.predecs)
	if err != nil {
		if evalErr, ok := err.(*starlark.EvalError); ok {
			return fmt.Errorf(evalErr.Backtrace())
		}
		return err
	}

	// add result to predeclared
	for k, v := range result {
		e.predecs[k] = v
	}

	return nil
}

func (e *Executor) Exec(name string, source io.Reader) error {
	if err := setupLocalDefaults(e.thread); err != nil {
		return fmt.Errorf("failed to setup defaults: %s", err)
	}

	result, err := starlark.ExecFile(e.thread, name, source, e.predecs)
	if err != nil {
		if evalErr, ok := err.(*starlark.EvalError); ok {
			return fmt.Errorf(evalErr.Backtrace())
		}
		return err
	}
	e.result = result

	// fetch and stop the instance of ssh-agent, if any
	if agentVal := e.thread.Local(identifiers.sshAgent); agentVal != nil {
		logrus.Debug("stopping ssh-agent")
		agent, ok := agentVal.(ssh.Agent)
		if !ok {
			logrus.Warn("error fetching ssh-agent")
		} else {
			if e := agent.Stop(); e != nil {
				logrus.Warnf("failed to stop ssh-agent: %v", e)
			}
		}
	}

	return nil
}

// setupLocalDefaults populates the provided execution thread
// with default configuration values.
func setupLocalDefaults(thread *starlark.Thread) error {
	if thread == nil {
		return errors.New("thread local is nil")
	}
	// add script context starlark thread
	ctx := context.Background()
	thread.SetLocal(identifiers.scriptCtx, ctx)

	// add default logger
	addDefaultLogger(thread)

	if err := addDefaultCrashdConf(thread); err != nil {
		return err
	}

	if err := addDefaultSSHConf(thread); err != nil {
		return err
	}

	if err := addDefaultKubeConf(thread); err != nil {
		return err
	}

	return nil
}

// newPredeclareds creates string dictionary containing the
// global built-ins values and functions available to the
// running script.
func newPredeclareds() starlark.StringDict {
	return starlark.StringDict{
		identifiers.os:                setupOSStruct(),
		identifiers.crashdCfg:         starlark.NewBuiltin(identifiers.crashdCfg, crashdConfigFn),
		identifiers.sshCfg:            starlark.NewBuiltin(identifiers.sshCfg, SshConfigFn),
		identifiers.hostListProvider:  starlark.NewBuiltin(identifiers.hostListProvider, hostListProvider),
		identifiers.resources:         starlark.NewBuiltin(identifiers.resources, resourcesFunc),
		identifiers.archive:           starlark.NewBuiltin(identifiers.archive, archiveFunc),
		identifiers.run:               starlark.NewBuiltin(identifiers.run, runFunc),
		identifiers.runLocal:          starlark.NewBuiltin(identifiers.runLocal, runLocalFunc),
		identifiers.progAvailLocal:    starlark.NewBuiltin(identifiers.progAvailLocal, progAvailLocalFunc),
		identifiers.capture:           starlark.NewBuiltin(identifiers.capture, captureFunc),
		identifiers.captureLocal:      starlark.NewBuiltin(identifiers.capture, captureLocalFunc),
		identifiers.copyFrom:          starlark.NewBuiltin(identifiers.copyFrom, copyFromFunc),
		identifiers.copyTo:            starlark.NewBuiltin(identifiers.copyTo, copyToFunc),
		identifiers.kubeCfg:           starlark.NewBuiltin(identifiers.kubeCfg, KubeConfigFn),
		identifiers.kubeCapture:       starlark.NewBuiltin(identifiers.kubeGet, KubeCaptureFn),
		identifiers.kubeGet:           starlark.NewBuiltin(identifiers.kubeGet, KubeGetFn),
		identifiers.kubeNodesProvider: starlark.NewBuiltin(identifiers.kubeNodesProvider, KubeNodesProviderFn),
		identifiers.capvProvider:      starlark.NewBuiltin(identifiers.capvProvider, CapvProviderFn),
		identifiers.capaProvider:      starlark.NewBuiltin(identifiers.capaProvider, CapaProviderFn),
		identifiers.setDefaults:       starlark.NewBuiltin(identifiers.setDefaults, SetDefaultsFunc),
		identifiers.log:               starlark.NewBuiltin(identifiers.log, logFunc),
	}
}
