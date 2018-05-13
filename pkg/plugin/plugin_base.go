/*
Copyright 2018 the Heptio Ark contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plugin

import "github.com/sirupsen/logrus"

type pluginBase struct {
	clientLogger logrus.FieldLogger
	*serverMux
}

func newPluginBase(options ...pluginOption) *pluginBase {
	base := new(pluginBase)
	for _, option := range options {
		option(base)
	}
	return base
}

type pluginOption func(base *pluginBase)

func clientLogger(logger logrus.FieldLogger) pluginOption {
	return func(base *pluginBase) {
		base.clientLogger = logger
	}
}

func serverLogger(logger logrus.FieldLogger) pluginOption {
	return func(base *pluginBase) {
		base.serverMux = newServerMux(logger)
	}
}
