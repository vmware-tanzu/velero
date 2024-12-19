// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vladimirvivien/gexe"
)

// ssh-agent constant identifiers
const (
	AgentPidIdentifier = "SSH_AGENT_PID"
	AuthSockIdentifier = "SSH_AUTH_SOCK"
)

type Agent interface {
	AddKey(keyPath string) error
	RemoveKey(keyPath string) error
	Stop() error
	GetEnvVariables() string
}

// agentInfo captures the connection information of the ssh-agent
type agentInfo map[string]string

// agent represents an instance of the ssh-agent
type agent struct {
	// Pid of the ssh-agent
	Pid string

	// Authentication socket to communicate with the ssh-agent
	AuthSockPath string

	// File paths of the keys added to the ssh-agent
	KeyPaths []string
}

// AddKey adds a key to the ssh-agent process
func (agent *agent) AddKey(keyPath string) error {
	e := gexe.New()
	sshAddCmd := e.Prog().Avail("ssh-add")
	if len(sshAddCmd) == 0 {
		return errors.New("ssh-add not found")
	}

	p := e.Envs(agent.GetEnvVariables()).
		RunProc(fmt.Sprintf("%s %s", sshAddCmd, keyPath))
	if err := p.Err(); err != nil {
		return errors.Wrapf(err, "could not add key %s to ssh-agent", keyPath)
	}
	agent.KeyPaths = append(agent.KeyPaths, keyPath)
	return nil
}

// RemoveKey removes a key from the ssh-agent process
func (agent *agent) RemoveKey(keyPath string) error {
	e := gexe.New()
	sshAddCmd := e.Prog().Avail("ssh-add")
	if len(sshAddCmd) == 0 {
		return errors.New("ssh-add not found")
	}

	p := e.Envs(agent.GetEnvVariables()).
		RunProc(fmt.Sprintf("%s -d %s", sshAddCmd, keyPath))
	if err := p.Err(); err != nil {
		return errors.Wrapf(err, "could not add key %s to ssh-agent", keyPath)
	}

	return nil
}

// Stop kills the ssh-agent process.
// It also tries to remove the added keys from the agent
func (agent *agent) Stop() error {
	for _, path := range agent.KeyPaths {
		logrus.Debugf("removing key from ssh-agent: %s", path)
		err := agent.RemoveKey(path)
		if err != nil {
			logrus.Warnf("failed to remove SSH key from agent: %s", err)
		}
	}

	logrus.Debugf("stopping the ssh-agent with Pid: %s", agent.Pid)
	p := gexe.Envs(agent.GetEnvVariables()).RunProc("ssh-agent -k")
	logrus.Debugf("ssh-agent stopped: %s", p.Result())

	return p.Err()
}

// GetEnvVariables returns the space separated key=value information used to communicate with the ssh-agent
func (agent *agent) GetEnvVariables() string {
	return fmt.Sprintf("%s=%s %s=%s", AgentPidIdentifier, agent.Pid, AuthSockIdentifier, agent.AuthSockPath)
}

// StartAgent starts the ssh-agent process and returns the SSH authentication parameters.
func StartAgent() (Agent, error) {
	e := gexe.New()
	sshAgentCmd := e.Prog().Avail("ssh-agent")
	if len(sshAgentCmd) == 0 {
		return nil, fmt.Errorf("ssh-agent not found")
	}

	logrus.Debugf("starting %s", sshAgentCmd)
	p := e.StartProc(fmt.Sprintf("%s -s", sshAgentCmd))
	if p.Err() != nil {
		return nil, errors.Wrap(p.Err(), "failed to start ssh agent")
	}

	agentInfo, err := parseAgentInfo(p.Out())
	if err != nil {
		return nil, err
	}
	if err := validateAgentInfo(agentInfo); err != nil {
		return nil, err
	}

	logrus.Debugf("ssh-agent started %v", agentInfo)
	return agentFromInfo(agentInfo), nil
}

// parseAgentInfo parses the output of ssh-agent -s to determine the information
// for the ssh authentication agent.
// example output:
//   SSH_AUTH_SOCK=/foo/bar.1234; export SSH_AUTH_SOCK;
//   SSH_AGENT_PID=4567; export SSH_AGENT_PID;
//   echo Agent pid 4567;
func parseAgentInfo(info io.Reader) (agentInfo, error) {
	agentInfo := map[string]string{}

	scanner := bufio.NewScanner(info)
	if err := scanner.Err(); err != nil {
		return agentInfo, err
	}

	for scanner.Scan() {
		line := scanner.Text()
		// separate the line using the semi-colon as a separator
		if equal := strings.Index(line, ";"); equal >= 0 {
			s := strings.Split(line, ";")[0]
			// check if any key=value pair is present
			if equal := strings.Index(s, "="); equal >= 0 {
				kv := strings.Split(s, "=")
				// store the key-value pair in the map
				agentInfo[kv[0]] = kv[1]
			}
		}
	}

	return agentInfo, nil
}

// validateAgentInfo checks whether the ssh-agent information is valid
func validateAgentInfo(info agentInfo) error {
	if len(info) != 2 {
		return errors.New("faulty ssh-agent identifier info")
	}
	for k, v := range info {
		if !strings.Contains(strings.Join([]string{AgentPidIdentifier, AuthSockIdentifier}, ""), k) || len(v) == 0 {
			return errors.New("faulty ssh-agent identifier info")
		}
	}
	return nil
}

// agentFromInfo parses the information map and returns an instance of agent
func agentFromInfo(agentInfo agentInfo) *agent {
	return &agent{
		Pid:          agentInfo[AgentPidIdentifier],
		AuthSockPath: agentInfo[AuthSockIdentifier],
	}
}
