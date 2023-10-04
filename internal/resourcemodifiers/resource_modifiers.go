package resourcemodifiers

import (
	"fmt"
	"regexp"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/gobwas/glob"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/vmware-tanzu/velero/pkg/util/collections"
)

const (
	ConfigmapRefType                   = "configmap"
	ResourceModifierSupportedVersionV1 = "v1"
)

type Conditions struct {
	Namespaces        []string              `json:"namespaces,omitempty"`
	GroupResource     string                `json:"groupResource"`
	ResourceNameRegex string                `json:"resourceNameRegex,omitempty"`
	LabelSelector     *metav1.LabelSelector `json:"labelSelector,omitempty"`
	Matches           []JSONPatch           `json:"matches,omitempty"`
}

type ResourceModifierRule struct {
	Conditions       Conditions            `json:"conditions"`
	PatchData        string                `json:"patchData,omitempty"`
	Patches          []JSONPatch           `json:"patches,omitempty"`
	MergePatches     []JSONMergePatch      `json:"mergePatches,omitempty"`
	StrategicPatches []StrategicMergePatch `json:"strategicPatches,omitempty"`
}

type ResourceModifiers struct {
	Version               string                 `json:"version"`
	ResourceModifierRules []ResourceModifierRule `json:"resourceModifierRules"`
}

func GetResourceModifiersFromConfig(cm *v1.ConfigMap) (*ResourceModifiers, error) {
	if cm == nil {
		return nil, fmt.Errorf("could not parse config from nil configmap")
	}
	if len(cm.Data) != 1 {
		return nil, fmt.Errorf("illegal resource modifiers %s/%s configmap", cm.Namespace, cm.Name)
	}

	var yamlData string
	for _, v := range cm.Data {
		yamlData = v
	}

	resModifiers, err := unmarshalResourceModifiers([]byte(yamlData))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return resModifiers, nil
}

func (p *ResourceModifiers) ApplyResourceModifierRules(obj *unstructured.Unstructured, groupResource string, scheme *runtime.Scheme, log logrus.FieldLogger) []error {
	var errs []error
	for _, rule := range p.ResourceModifierRules {
		err := rule.apply(obj, groupResource, scheme, log)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

func (r *ResourceModifierRule) apply(obj *unstructured.Unstructured, groupResource string, scheme *runtime.Scheme, log logrus.FieldLogger) error {
	namespaceInclusion := collections.NewIncludesExcludes().Includes(r.Conditions.Namespaces...)
	if !namespaceInclusion.ShouldInclude(obj.GetNamespace()) {
		return nil
	}

	g, err := glob.Compile(r.Conditions.GroupResource)
	if err != nil {
		log.Errorf("bad glob pattern %s, err: %s", r.Conditions.GroupResource, err)
		return err
	}

	if !g.Match(groupResource) {
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

	if r.Conditions.LabelSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(r.Conditions.LabelSelector)
		if err != nil {
			return errors.Errorf("error in creating label selector %s", err.Error())
		}
		if !selector.Matches(labels.Set(obj.GetLabels())) {
			return nil
		}
	}

	match, err := matchConditions(obj, r.Conditions.Matches, log)
	if err != nil {
		return err
	} else if !match {
		log.Info("Conditions do not match, skip it")
		return nil
	}

	log.Infof("Applying resource modifier patch on %s/%s", obj.GetNamespace(), obj.GetName())
	err = r.applyPatch(obj, scheme, log)
	if err != nil {
		return err
	}
	return nil
}

func matchConditions(u *unstructured.Unstructured, patches []JSONPatch, _ logrus.FieldLogger) (bool, error) {
	if len(patches) == 0 {
		return true, nil
	}

	var fixed []JSONPatch
	for _, patch := range patches {
		patch.From = ""
		patch.Operation = "test"
		fixed = append(fixed, patch)
	}

	p := &JSONPatcher{patches: fixed}
	_, err := p.applyPatch(u)
	if err != nil {
		if errors.Is(err, jsonpatch.ErrTestFailed) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func unmarshalResourceModifiers(yamlData []byte) (*ResourceModifiers, error) {
	resModifiers := &ResourceModifiers{}
	err := yaml.UnmarshalStrict(yamlData, resModifiers)
	if err != nil {
		return nil, fmt.Errorf("failed to decode yaml data into resource modifiers  %v", err)
	}
	return resModifiers, nil
}

type patcher interface {
	Patch(u *unstructured.Unstructured, logger logrus.FieldLogger) (*unstructured.Unstructured, error)
}

func (r *ResourceModifierRule) applyPatch(u *unstructured.Unstructured, scheme *runtime.Scheme, logger logrus.FieldLogger) error {
	var p patcher
	if len(r.Patches) > 0 {
		p = &JSONPatcher{patches: r.Patches}
	} else if len(r.MergePatches) > 0 {
		p = &JSONMergePatcher{patches: r.MergePatches}
	} else if len(r.StrategicPatches) > 0 {
		p = &StrategicMergePatcher{patches: r.StrategicPatches, scheme: scheme}
	} else {
		return fmt.Errorf("no patch data found")
	}

	updated, err := p.Patch(u, logger)
	if err != nil {
		return fmt.Errorf("error in applying patch %s", err)
	}

	u.SetUnstructuredContent(updated.Object)
	return nil
}
