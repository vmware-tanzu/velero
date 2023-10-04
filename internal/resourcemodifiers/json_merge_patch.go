package resourcemodifiers

import (
	"fmt"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type JSONMergePatch struct {
	PatchBytes []byte `json:"patchBytes,omitempty"`
}

type JSONMergePatcher struct {
	patches []JSONMergePatch
}

func (p *JSONMergePatcher) Patch(u *unstructured.Unstructured, _ logrus.FieldLogger) (*unstructured.Unstructured, error) {
	objBytes, err := u.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("error in marshaling object %s", err)
	}

	for _, patch := range p.patches {
		patchBytes, err := yaml.YAMLToJSON(patch.PatchBytes)
		if err != nil {
			return nil, fmt.Errorf("error in converting YAML to JSON %s", err)
		}

		objBytes, err = jsonpatch.MergePatch(objBytes, patchBytes)
		if err != nil {
			return nil, fmt.Errorf("error in applying JSON Patch %s", err.Error())
		}
	}

	updated := &unstructured.Unstructured{}
	err = updated.UnmarshalJSON(objBytes)
	if err != nil {
		return nil, fmt.Errorf("error in unmarshalling modified object %s", err.Error())
	}

	return updated, nil
}
