package resourcepolicies

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
)

type VolumeActionType string

const Skip VolumeActionType = "skip"

// Action defined as one action for a specific way of backup
type Action struct {
	// Type defined specific type of action, currently only support 'skip'
	Type VolumeActionType `yaml:"type"`
	// Parameters defined map of parameters when executing a specific action
	Parameters map[string]interface{} `yaml:"parameters,omitempty"`
}

// VolumePolicy defined policy to conditions to match Volumes and related action to handle matched Volumes
type VolumePolicy struct {
	// Conditions defined list of conditions to match Volumes
	Conditions map[string]interface{} `yaml:"conditions"`
	Action     Action                 `yaml:"action"`
}

// currently only support configmap type of resource config
const ConfigmapRefType string = "configmap"

// ResourcePolicies currently defined slice of volume policies to handle backup
type ResourcePolicies struct {
	Version        string         `yaml:"version"`
	VolumePolicies []VolumePolicy `yaml:"volumePolicies"`
	// we may support other resource policies in the future, and they could be added separately
	// OtherResourcePolicies: []OtherResourcePolicy
}

type resourcePolicyMatcher interface {
	Match(res interface{}) *Action
}

type volumePolicyMatcherImpl struct {
	policy      *VolumePolicy
	matcherFunc func(volume *StructuredVolume) *Action
}

func (v *volumePolicyMatcherImpl) Match(volume interface{}) *Action {
	val, ok := volume.(*StructuredVolume)
	if !ok {
		return nil
	}
	return v.matcherFunc(val)
}

// Match interface is implemented by VolumePolicy
func (policy *VolumePolicy) Match(volume *StructuredVolume) *Action {
	matcher := policyConditionsMatcher{}
	matcher.addPolicy(policy)
	return matcher.Match(volume)
}

func newVolumePolicyMatcher(policy *VolumePolicy) resourcePolicyMatcher {
	return &volumePolicyMatcherImpl{
		policy:      policy,
		matcherFunc: policy.Match,
	}
}

type resourcePoliciesMatcherFactory struct {
	volumeMatchers []resourcePolicyMatcher
	// could extended by add other policy matcher
	// otherResourceMatchers   []resourcePolicyMatcher
}

func (r *resourcePoliciesMatcherFactory) addVolumePolicyMatcher(volumePolicy []VolumePolicy) {
	for k := range volumePolicy {
		matcher := newVolumePolicyMatcher(&volumePolicy[k])
		r.volumeMatchers = append(r.volumeMatchers, matcher)
	}
}

func newResourcePoliciesMatcherFactory(policies *ResourcePolicies) *resourcePoliciesMatcherFactory {
	factory := &resourcePoliciesMatcherFactory{}
	factory.addVolumePolicyMatcher(policies.VolumePolicies)
	// TODO we can add other policy matcher into the factory
	// factory.AddOtherResourcePolicyMatcher(policies.OtherResourcePolicies)
	return factory
}

// getMatchers return related resource policy matcher
func (r *resourcePoliciesMatcherFactory) getMatchers(resourceType string) []resourcePolicyMatcher {
	switch resourceType {
	case "volume":
		return r.volumeMatchers
	default:
		return nil
	}
}

func unmarshalResourcePolicies(YamlData *string) (*ResourcePolicies, error) {
	resPolicies := &ResourcePolicies{}
	if err := decodeStruct(strings.NewReader(*YamlData), resPolicies); err != nil {
		return nil, fmt.Errorf("failed to decode yaml data into resource policies  %v", err)
	} else {
		return resPolicies, nil
	}
}

// GetVolumeMatchedAction checks the current volume is match resource policies
// It will return once matched ignoring the latter policies
func GetVolumeMatchedAction(policies *ResourcePolicies, volume *StructuredVolume) *Action {
	factory := newResourcePoliciesMatcherFactory(policies)
	matchers := factory.getMatchers("volume")

	for _, matcher := range matchers {
		action := matcher.Match(volume)
		if action != nil {
			return action
		}
	}
	return nil
}

func GetResourcePoliciesFromConfig(cm *v1.ConfigMap) (*ResourcePolicies, error) {
	if cm == nil {
		return nil, fmt.Errorf("could not parse config from nil configmap")
	}
	if len(cm.Data) != 1 {
		return nil, fmt.Errorf("illegal resource policies %s/%s configmap", cm.Name, cm.Namespace)
	}
	var yamlData string
	for _, v := range cm.Data {
		yamlData = v
	}
	return unmarshalResourcePolicies(&yamlData)
}
