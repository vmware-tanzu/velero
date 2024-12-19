// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/vladimirvivien/gexe"
	"github.com/vladimirvivien/gexe/exec"
	"k8s.io/apimachinery/pkg/util/wait"
)

type ProxyJumpArgs struct {
	User string
	Host string
}

type SSHArgs struct {
	User           string
	Host           string
	PrivateKeyPath string
	Port           string
	MaxRetries     int
	ProxyJump      *ProxyJumpArgs
}

// Run runs a command over SSH and returns the result as a string
func Run(args SSHArgs, agent Agent, cmd string) (string, error) {
	reader, err := sshRunProc(args, agent, cmd)
	if err != nil {
		return "", err
	}
	var result bytes.Buffer
	if _, err := result.ReadFrom(reader); err != nil {
		return "", err
	}
	return strings.TrimSpace(result.String()), nil
}

// RunRead runs a command over SSH and returns an io.Reader for stdout/stderr
func RunRead(args SSHArgs, agent Agent, cmd string) (io.Reader, error) {
	return sshRunProc(args, agent, cmd)
}

func sshRunProc(args SSHArgs, agent Agent, cmd string) (io.Reader, error) {
	e := gexe.New()
	prog := e.Prog().Avail("ssh")
	if len(prog) == 0 {
		return nil, fmt.Errorf("ssh program not found")
	}

	sshCmd, err := makeSSHCmdStr(prog, args)
	if err != nil {
		return nil, err
	}
	effectiveCmd := fmt.Sprintf(`%s "%s"`, sshCmd, cmd)
	logrus.Debug("ssh.run: ", effectiveCmd)

	if agent != nil {
		logrus.Debugf("Adding agent info: %s", agent.GetEnvVariables())
		e = e.Envs(agent.GetEnvVariables())
	}

	var proc *exec.Proc
	maxRetries := args.MaxRetries
	if maxRetries == 0 {
		maxRetries = 10
	}
	retries := wait.Backoff{Steps: maxRetries, Duration: time.Millisecond * 80, Jitter: 0.1}
	if err := wait.ExponentialBackoff(retries, func() (bool, error) {
		p := e.StartProc(effectiveCmd)
		if p.Err() != nil {
			logrus.Warn(fmt.Sprintf("ssh: failed to connect to %s: error '%s %s': retrying connection", args.Host, p.Err(), p.Result()))
			return false, p.Err()
		}
		proc = p
		return true, nil // worked
	}); err != nil {
		logrus.Debugf("ssh.run failed after %d tries", maxRetries)
		return nil, fmt.Errorf("ssh: failed after %d attempt(s): %s", maxRetries, err)
	}

	if proc == nil {
		return nil, fmt.Errorf("ssh.run: did get process result")
	}

	return proc.Out(), nil
}

func makeSSHCmdStr(progName string, args SSHArgs) (string, error) {
	if args.User == "" {
		return "", fmt.Errorf("SSH: user is required")
	}
	if args.Host == "" {
		return "", fmt.Errorf("SSH: host is required")
	}

	if args.ProxyJump != nil {
		if args.ProxyJump.User == "" || args.ProxyJump.Host == "" {
			return "", fmt.Errorf("SSH: jump user and host are required")
		}
	}

	sshCmdPrefix := func() string {
		return fmt.Sprintf("%s -q -o StrictHostKeyChecking=no", progName)
	}

	pkPath := func() string {
		if args.PrivateKeyPath != "" {
			return fmt.Sprintf("-i %s", args.PrivateKeyPath)
		}
		return ""
	}

	port := func() string {
		if args.Port == "" {
			return "-p 22"
		}
		return fmt.Sprintf("-p %s", args.Port)
	}

	proxyJump := func() string {
		if args.ProxyJump != nil {
			return fmt.Sprintf("%s@%s", args.User, args.Host) + ` -o "ProxyCommand ssh -o StrictHostKeyChecking=no -W %h:%p ` + fmt.Sprintf("%s %s@%s\"", pkPath(), args.ProxyJump.User, args.ProxyJump.Host)
		}
		return ""
	}

	// build command as
	// ssh -i <pkpath> -P <port> user@host OR
	// ssh -i <pkpath> -P <port> user@host -o "ProxyCommand ssh -W %h:%p -i <pkpath> <proxyJump>"
	cmd := func() string {
		cmdStr := fmt.Sprintf("%s %s %s ", sshCmdPrefix(), pkPath(), port())

		if proxyDetails := proxyJump(); proxyDetails != "" {
			cmdStr += proxyDetails
		} else {
			cmdStr += fmt.Sprintf("%s@%s", args.User, args.Host)
		}

		return cmdStr
	}

	return cmd(), nil
}
