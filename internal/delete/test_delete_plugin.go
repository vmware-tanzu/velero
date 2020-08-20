/*
Copyright 2020 the Velero contributors.

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
package delete

import (
	"github.com/sirupsen/logrus"

	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

var _ velero.DeleteItemAction = (*DeletePlugin)(nil)

type DeletePlugin struct {
	log logrus.FieldLogger
}

func NewDeletePlugin(log logrus.FieldLogger) *DeletePlugin {
	return &DeletePlugin{log: log}
}

func (p *DeletePlugin) AppliesTo() (velero.ResourceSelector, error) {
	p.log.Debug("In DeletePlugin AppliesTo")
	return velero.ResourceSelector{}, nil
}

func (p *DeletePlugin) Execute(input *velero.DeleteItemActionExecuteInput) error {
	p.log.Debug("In DeletePlugin Execute")
	return nil
}
