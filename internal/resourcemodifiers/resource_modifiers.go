package resourcemodifiers

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/vmware-tanzu/velero/pkg/util/collections"
)

const (
	ConfigmapRefType                   = "configmap"
	ResourceModifierSupportedVersionV1 = "v1"
)

type JSONPatch struct {
	Operation string `yaml:"operation"`
	From      string `yaml:"from,omitempty"`
	Path      string `yaml:"path"`
	Value     string `yaml:"value,omitempty"`
}

type Conditions struct {
	Namespaces        []string `yaml:"namespaces,omitempty"`
	GroupKind         string   `yaml:"groupKind"`
	ResourceNameRegex string   `yaml:"resourceNameRegex"`
}

type ResourceModifierRule struct {
	Conditions Conditions  `yaml:"conditions"`
	Patches    []JSONPatch `yaml:"patches"`
}

type ResourceModifiers struct {
	Version               string                 `yaml:"version"`
	ResourceModifierRules []ResourceModifierRule `yaml:"resourceModifierRules"`
}

func GetResourceModifiersFromConfig(cm *v1.ConfigMap) (*ResourceModifiers, error) {
	if cm == nil {
		return nil, fmt.Errorf("could not parse config from nil configmap")
	}
	if len(cm.Data) != 1 {
		return nil, fmt.Errorf("illegal resource modifiers %s/%s configmap", cm.Name, cm.Namespace)
	}

	var yamlData string
	for _, v := range cm.Data {
		yamlData = v
	}

	resModifiers, err := unmarshalResourceModifiers(&yamlData)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return resModifiers, nil
}

func (p *ResourceModifiers) ApplyResourceModifierRules(obj *unstructured.Unstructured, groupResource string, log logrus.FieldLogger) []error {
	var errs []error
	for _, rule := range p.ResourceModifierRules {
		err := rule.Apply(obj, groupResource, log)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

func (r *ResourceModifierRule) Apply(obj *unstructured.Unstructured, groupResource string, log logrus.FieldLogger) error {
	namespaceInclusion := collections.NewIncludesExcludes().Includes(r.Conditions.Namespaces...)
	if !namespaceInclusion.ShouldInclude(obj.GetNamespace()) {
		return nil
	}
	if !strings.EqualFold(groupResource, r.Conditions.GroupKind) {
		return nil
	}
	if r.Conditions.ResourceNameRegex != "" {
		match, err := regexp.MatchString(r.Conditions.ResourceNameRegex, obj.GetName())
		if err != nil {
			return errors.Errorf("error in matching regex %s", err.Error())
		}
		if !match {
			return nil
		}
	}
	patches, err := r.PatchArrayToByteArray()
	if err != nil {
		return err
	}
	log.Infof("Applying resource modifier patch on %s/%s", obj.GetNamespace(), obj.GetName())
	err = ApplyPatch(patches, obj, log)
	if err != nil {
		return err
	}
	return nil
}

// convert all JsonPatch to string array with the format of jsonpatch.Patch and then convert it to byte array
func (r *ResourceModifierRule) PatchArrayToByteArray() ([]byte, error) {
	var patches []string
	for _, patch := range r.Patches {
		patches = append(patches, patch.ToString())
	}
	patchesStr := strings.Join(patches, ",\n\t")
	return []byte(fmt.Sprintf(`[%s]`, patchesStr)), nil
}

func (p *JSONPatch) ToString() string {
	if strings.Contains(p.Value, "\"") {
		return fmt.Sprintf(`{"op": "%s", "from": "%s", "path": "%s", "value": %s}`, p.Operation, p.From, p.Path, p.Value)
	}
	return fmt.Sprintf(`{"op": "%s", "from": "%s", "path": "%s", "value": "%s"}`, p.Operation, p.From, p.Path, p.Value)
}

func ApplyPatch(patch []byte, obj *unstructured.Unstructured, log logrus.FieldLogger) error {
	jsonPatch, err := jsonpatch.DecodePatch(patch)
	if err != nil {
		return fmt.Errorf("error in decoding json patch %s", err.Error())
	}
	objBytes, err := obj.MarshalJSON()
	if err != nil {
		return fmt.Errorf("error in marshaling object %s", err.Error())
	}
	modifiedObjBytes, err := jsonPatch.Apply(objBytes)
	if err != nil {
		if errors.Is(err, jsonpatch.ErrTestFailed) {
			log.Infof("Test operation failed for JSON Patch %s", err.Error())
			return nil
		}
		return fmt.Errorf("error in applying JSON Patch %s", err.Error())
	}
	err = obj.UnmarshalJSON(modifiedObjBytes)
	if err != nil {
		return fmt.Errorf("error in unmarshalling modified object %s", err.Error())
	}
	return nil
}

func unmarshalResourceModifiers(yamlData *string) (*ResourceModifiers, error) {
	resModifiers := &ResourceModifiers{}
	err := decodeStruct(strings.NewReader(*yamlData), resModifiers)
	if err != nil {
		return nil, fmt.Errorf("failed to decode yaml data into resource modifiers  %v", err)
	}
	return resModifiers, nil
}

// decodeStruct restrict validate the keys in decoded mappings to exist as fields in the struct being decoded into
func decodeStruct(r io.Reader, s interface{}) error {
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	return dec.Decode(s)
}
