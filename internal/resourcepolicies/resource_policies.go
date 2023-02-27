package resourcepolicies

import (
	"fmt"
	"strings"
)

// Action defined as one action for a specific way of backup
type Action struct {
	// Type defined specific type of action, currently only support 'skip'
	Type string `yaml:"type"`
	// Parameters defined map of parameters when executing a specific action
	// +optional
	// +nullable
	Parameters map[string]interface{} `yaml:"parameters,omitempty"`
}

// VolumePolicy defined policy to conditions to match Volumes and related action to handle matched Volumes
type VolumePolicy struct {
	// Conditions defined list of conditions to match Volumes
	Conditions map[string]interface{} `yaml:"conditions"`
	Action     Action                 `yaml:"action"`
}

// ResourcePolicies currently defined slice of volume policies to handle backup
type ResourcePolicies struct {
	Version        string         `yaml:"version"`
	VolumePolicies []VolumePolicy `yaml:"volumePolicies"`
	// we may support other resource policies in the future, and they could be added separately
	// OtherResourcePolicies: []OtherResourcePolicy
}

type resourcePolicyMatcher interface {
	Match(res interface{}, actionType string) bool
}

type volumePolicyMatcherImpl struct {
	policy      *VolumePolicy
	matcherFunc func(volume interface{}, actionType string) bool
}

func (v *volumePolicyMatcherImpl) Match(volume interface{}, actionType string) bool {
	return v.matcherFunc(volume, actionType)
}

// Match interface is implemented by VolumePolicy
func (policy *VolumePolicy) Match(volume interface{}, actionType string) bool {
	val, ok := volume.(*StructuredVolume)
	if !ok {
		return false
	}
	matcher := policyConditionsMatcher{}
	matcher.addPolicy(policy)
	return matcher.Match(val) == policy.Action.Type
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

func LoadResourcePolicies(YamlData *string) (*ResourcePolicies, error) {
	resPolicies := &ResourcePolicies{}
	if err := decodeStruct(strings.NewReader(*YamlData), resPolicies); err != nil {
		return nil, fmt.Errorf("failed to decode yaml data into resource policies  %v", err)
	} else {
		return resPolicies, nil
	}
}

// GetVolumeMatchedAction checks the current volume is match resource policies
// It will return once matched ignoring the latter policies
func GetVolumeMatchedAction(policies *ResourcePolicies, volume *StructuredVolume, actionType string) string {
	factory := newResourcePoliciesMatcherFactory(policies)
	matchers := factory.getMatchers("volume")

	for _, matcher := range matchers {
		if matcher.Match(volume, actionType) {
			return actionType
		}
	}
	return ""
}
