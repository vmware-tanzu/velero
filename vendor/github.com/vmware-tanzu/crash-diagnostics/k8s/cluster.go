// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"errors"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type Config interface {
	GetClusterName() (string, error)
	GetCurrentContext() string
}

type KubeConfig struct {
	config *clientcmdapi.Config
}

func (kcfg *KubeConfig) GetClusterName() (string, error) {
	currCtx := kcfg.GetCurrentContext()
	if ctx, ok := kcfg.config.Contexts[currCtx]; ok {
		return ctx.Cluster, nil
	} else {
		return "", errors.New("unknown context: " + currCtx)
	}
}

func (kcfg *KubeConfig) GetCurrentContext() string {
	return kcfg.config.CurrentContext
}

func LoadKubeCfg(kubeConfigPath string) (Config, error) {
	cfg, err := clientcmd.LoadFromFile(kubeConfigPath)
	if err != nil {
		return nil, err
	}
	return &KubeConfig{config: cfg}, nil
}
