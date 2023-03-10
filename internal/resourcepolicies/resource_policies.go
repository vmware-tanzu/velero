package resourcepolicies

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
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

type Policies struct {
	Version        string
	VolumePolicies []volumePolicy
	// OtherPolicies
}

func unmarshalResourcePolicies(YamlData *string) (*ResourcePolicies, error) {
	resPolicies := &ResourcePolicies{}
	if err := decodeStruct(strings.NewReader(*YamlData), resPolicies); err != nil {
		return nil, fmt.Errorf("failed to decode yaml data into resource policies  %v", err)
	} else {
		return resPolicies, nil
	}
}

func (policies *Policies) buildPolicy(resPolicies *ResourcePolicies) error {
	for _, vp := range resPolicies.VolumePolicies {
		con, err := unmarshalVolConditions(vp.Conditions)
		if err != nil {
			return errors.Wrap(err, "failed to unmarshl volume conditions")
		}
		volCap, err := parseCapacity(con.Capacity)
		if err != nil {
			return errors.Wrapf(err, "failed to parse condition capacity %s", con.Capacity)
		}
		var p volumePolicy
		p.action = vp.Action
		p.conditions = append(p.conditions, &capacityCondition{capacity: *volCap})
		p.conditions = append(p.conditions, &storageClassCondition{storageClass: con.StorageClass})
		p.conditions = append(p.conditions, &nfsCondition{nfs: con.NFS})
		p.conditions = append(p.conditions, &csiCondition{csi: con.CSI})
		policies.VolumePolicies = append(policies.VolumePolicies, p)
	}

	// Other resource policies

	policies.Version = resPolicies.Version
	return nil
}

func (p *Policies) Match(res interface{}) *Action {
	vol, ok := res.(*StructuredVolume)
	if ok {
		for _, policy := range p.VolumePolicies {
			isAllMatch := false
			for _, con := range policy.conditions {
				if !con.Match(vol) {
					isAllMatch = false
					break
				}
				isAllMatch = true
			}
			if isAllMatch {
				return &policy.action
			}
		}
	}
	return nil
}

func (p *Policies) Validate() error {
	if p.Version != currentSupportDataVersion {
		return fmt.Errorf("incompatible version number %s with supported version %s", p.Version, currentSupportDataVersion)
	}

	for _, policy := range p.VolumePolicies {
		if err := policy.action.Validate(); err != nil {
			return errors.Wrap(err, "failed to validate config")
		}
		for _, con := range policy.conditions {
			if err := con.Validate(); err != nil {
				return errors.Wrap(err, "failed to validate conditions config")
			}
		}
	}
	return nil
}

func GetResourcePoliciesFromConfig(cm *v1.ConfigMap) (*Policies, error) {
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

	resPolicies, err := unmarshalResourcePolicies(&yamlData)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	policies := &Policies{}
	if err := policies.buildPolicy(resPolicies); err != nil {
		return nil, errors.WithStack(err)
	}

	return policies, nil
}
