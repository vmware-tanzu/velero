/*
Copyright the Velero contributors.

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

package report

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/vmware-tanzu/velero/test"
)

func GenerateYamlReport() error {
	yamlData, err := yaml.Marshal(test.ReportData)
	if err != nil {
		return errors.Wrap(err, "error marshal data into yaml")
	}
	// Writing the XML output to a file
	err = ioutil.WriteFile("perf-report.yaml", yamlData, 0644)
	if err != nil {
		return errors.Wrap(err, "error writing yaml to file")
	}
	return nil
}

func AddTestSuitData(dataMap map[string]interface{}, testSuitDesc string) {
	test.ReportData.OtherFields[testSuitDesc] = dataMap
}
