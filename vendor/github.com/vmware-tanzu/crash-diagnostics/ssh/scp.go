// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/vladimirvivien/gexe"
	"k8s.io/apimachinery/pkg/util/wait"
)

// CopyFrom copies one or more files using SCP from remote host
// and returns the paths of files that were successfully copied.
func CopyFrom(args SSHArgs, agent Agent, rootDir string, sourcePath string) error {
	e := gexe.New()
	prog := e.Prog().Avail("scp")
	if len(prog) == 0 {
		return fmt.Errorf("scp program not found")
	}

	targetPath := filepath.Join(rootDir, sourcePath)
	targetDir := filepath.Dir(targetPath)
	pathDir, pathFile := filepath.Split(sourcePath)
	if strings.Contains(pathFile, "*") {
		targetPath = filepath.Join(rootDir, pathDir)
		targetDir = targetPath
	}

	if err := os.MkdirAll(targetDir, 0744); err != nil && !os.IsExist(err) {
		return err
	}

	sshCmd, err := makeSCPCmdStr(prog, args)
	if err != nil {
		return fmt.Errorf("scp: copyFrom: failed to build command string: %s", err)
	}

	effectiveCmd := fmt.Sprintf(`%s %s`, sshCmd, getCopyFromSourceTarget(args, sourcePath, targetPath))
	logrus.Debugf("scp: copFrom: cmd: [%s]", effectiveCmd)

	if agent != nil {
		logrus.Debugf("scp: copyFrom: adding agent info: %s", agent.GetEnvVariables())
		e = e.Envs(agent.GetEnvVariables())
	}

	maxRetries := args.MaxRetries
	if maxRetries == 0 {
		maxRetries = 10
	}
	retries := wait.Backoff{Steps: maxRetries, Duration: time.Millisecond * 80, Jitter: 0.1}
	if err := wait.ExponentialBackoff(retries, func() (bool, error) {
		p := e.RunProc(effectiveCmd)
		if p.Err() != nil {
			logrus.Warn(fmt.Sprintf("scp: copyFrom: failed to connect to %s: '%s %s': retrying connection", args.Host, p.Err(), p.Result()))
			return false, nil
		}
		return true, nil // worked
	}); err != nil {
		return fmt.Errorf("scp: copyFrom: failed after %d attempt(s): %s", maxRetries, err)
	}

	logrus.Debugf("scp: copyFrom: copied %s", sourcePath)
	return nil
}

// CopyTo copies one or more files using SCP from local machine to
// remote host.
func CopyTo(args SSHArgs, agent Agent, sourcePath, targetPath string) error {
	e := gexe.New()
	prog := e.Prog().Avail("scp")
	if len(prog) == 0 {
		return fmt.Errorf("scp program not found")
	}

	if len(sourcePath) == 0 {
		return fmt.Errorf("scp: copyTo: missing source path")
	}

	if len(targetPath) == 0 {
		return fmt.Errorf("scp: copyTo: missing target path")
	}

	sshCmd, err := makeSCPCmdStr(prog, args)
	if err != nil {
		return fmt.Errorf("scp: copyTo: failed to build command string: %s", err)
	}

	effectiveCmd := fmt.Sprintf(`%s %s`, sshCmd, getCopyToSourceTarget(args, sourcePath, targetPath))
	logrus.Debugf("scp: copyTo: cmd: [%s]", effectiveCmd)

	if agent != nil {
		logrus.Debugf("scp: adding agent info: %s", agent.GetEnvVariables())
		e = e.Envs(agent.GetEnvVariables())
	}

	maxRetries := args.MaxRetries
	if maxRetries == 0 {
		maxRetries = 10
	}
	retries := wait.Backoff{Steps: maxRetries, Duration: time.Millisecond * 80, Jitter: 0.1}
	if err := wait.ExponentialBackoff(retries, func() (bool, error) {
		p := e.RunProc(effectiveCmd)
		if p.Err() != nil {
			logrus.Warn(fmt.Sprintf("scp: failed to connect to %s: '%s %s': retrying connection", args.Host, p.Err(), p.Result()))
			return false, nil
		}
		return true, nil // worked
	}); err != nil {
		return fmt.Errorf("scp: copyTo: failed after %d attempt(s): %s", maxRetries, err)
	}

	logrus.Debugf("scp: copyTo: copied %s -> %s", sourcePath, targetPath)
	return nil
}

func makeSCPCmdStr(progName string, args SSHArgs) (string, error) {
	if args.User == "" {
		return "", fmt.Errorf("scp: user is required")
	}
	if args.Host == "" {
		return "", fmt.Errorf("scp: host is required")
	}

	if args.ProxyJump != nil {
		if args.ProxyJump.User == "" || args.ProxyJump.Host == "" {
			return "", fmt.Errorf("scp: jump user and host are required")
		}
	}

	scpCmdPrefix := func() string {
		return fmt.Sprintf("%s -rpq -o StrictHostKeyChecking=no", progName)
	}

	pkPath := func() string {
		if args.PrivateKeyPath != "" {
			return fmt.Sprintf("-i %s", args.PrivateKeyPath)
		}
		return ""
	}

	port := func() string {
		if args.Port == "" {
			return "-P 22"
		}
		return fmt.Sprintf("-P %s", args.Port)
	}

	proxyJump := func() string {
		if args.ProxyJump != nil {
			return fmt.Sprintf("-J %s@%s", args.ProxyJump.User, args.ProxyJump.Host)
		}
		return ""
	}
	// build command as
	// scp -i <pkpath> -P <port> -J <proxyjump> user@host:path
	cmd := fmt.Sprintf(
		`%s %s %s %s`,
		scpCmdPrefix(), pkPath(), port(), proxyJump(),
	)
	return cmd, nil
}

func getCopyFromSourceTarget(args SSHArgs, sourcePath, targetPath string) string {
	return fmt.Sprintf("%s@%s:%s %s", args.User, args.Host, sourcePath, targetPath)
}

func getCopyToSourceTarget(args SSHArgs, sourcePath, targetPath string) string {
	return fmt.Sprintf("%s %s@%s:%s", sourcePath, args.User, args.Host, targetPath)
}
