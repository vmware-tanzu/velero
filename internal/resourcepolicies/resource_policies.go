package resourcepolicies

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
)

type VolumeActionType string

const (
	// currently only support configmap type of resource config
	ConfigmapRefType string           = "configmap"
	Skip             VolumeActionType = "skip"
)

// Action defined as one action for a specific way of backup
type Action struct {
	// Type defined specific type of action, currently only support 'skip'
	Type VolumeActionType `yaml:"type"`
	// Parameters defined map of parameters when executing a specific action
	Parameters map[string]interface{} `yaml:"parameters,omitempty"`
}

// volumePolicy defined policy to conditions to match Volumes and related action to handle matched Volumes
type volumePolicy struct {
	// Conditions defined list of conditions to match Volumes
	Conditions map[string]interface{} `yaml:"conditions"`
	Action     Action                 `yaml:"action"`
}

// resourcePolicies currently defined slice of volume policies to handle backup
type resourcePolicies struct {
	Version        string         `yaml:"version"`
	VolumePolicies []volumePolicy `yaml:"volumePolicies"`
	// we may support other resource policies in the future, and they could be added separately
	// OtherResourcePolicies []OtherResourcePolicy
}

type Policies struct {
	version        string
	volumePolicies []volPolicy
	// OtherPolicies
}

func unmarshalResourcePolicies(yamlData *string) (*resourcePolicies, error) {
	resPolicies := &resourcePolicies{}
	if err := decodeStruct(strings.NewReader(*yamlData), resPolicies); err != nil {
		return nil, fmt.Errorf("failed to decode yaml data into resource policies  %v", err)
	} else {
		return resPolicies, nil
	}
}

func (policies *Policies) buildPolicy(resPolicies *resourcePolicies) error {
	for _, vp := range resPolicies.VolumePolicies {
		con, err := unmarshalVolConditions(vp.Conditions)
		if err != nil {
			return errors.WithStack(err)
		}
		volCap, err := parseCapacity(con.Capacity)
		if err != nil {
			return errors.WithStack(err)
		}
		var p volPolicy
		p.action = vp.Action
		p.conditions = append(p.conditions, &capacityCondition{capacity: *volCap})
		p.conditions = append(p.conditions, &storageClassCondition{storageClass: con.StorageClass})
		p.conditions = append(p.conditions, &nfsCondition{nfs: con.NFS})
		p.conditions = append(p.conditions, &csiCondition{csi: con.CSI})
		policies.volumePolicies = append(policies.volumePolicies, p)
	}

	// Other resource policies

	policies.version = resPolicies.Version
	return nil
}

func (p *Policies) match(res *structuredVolume) *Action {
	for _, policy := range p.volumePolicies {
		isAllMatch := false
		for _, con := range policy.conditions {
			if !con.match(res) {
				isAllMatch = false
				break
			}
			isAllMatch = true
		}
		if isAllMatch {
			return &policy.action
		}
	}
	return nil
}

func (p *Policies) GetMatchAction(res interface{}) (*Action, error) {
	volume := &structuredVolume{}
	switch obj := res.(type) {
	case *v1.PersistentVolume:
		volume.parsePV(obj)
	case *v1.Volume:
		volume.parsePodVolume(obj)
	default:
		return nil, errors.New("failed to convert object")
	}
	return p.match(volume), nil
}

func (p *Policies) Validate() error {
	if p.version != currentSupportDataVersion {
		return fmt.Errorf("incompatible version number %s with supported version %s", p.version, currentSupportDataVersion)
	}

	for _, policy := range p.volumePolicies {
		if err := policy.action.validate(); err != nil {
			return errors.WithStack(err)
		}
		for _, con := range policy.conditions {
			if err := con.validate(); err != nil {
				return errors.WithStack(err)
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
