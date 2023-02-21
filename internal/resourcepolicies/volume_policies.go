package resourcepolicies

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/api/resource"
)

type VolumePolicier struct {
	action        Action
	volConditions []VolumeCondition
}

type VolumeCondition interface {
	Validate() (bool, error)
	Match(v *StructuredVolume) bool
}

type ConditionsMatcher interface {
	Validate() (bool, error)
	// return matched action type
	Match(v *StructuredVolume) string
}

type PolicyConditionsMatcher struct {
	policies []*VolumePolicier
}

type ResourcePolicyMatcher interface {
	Match(res interface{}, actionType string) bool
}

// Capacity consist of the lower and upper boundary
type Capacity struct {
	lower resource.Quantity
	upper resource.Quantity
}

type StructuredVolume struct {
	capacity     resource.Quantity
	storageClass string
	nfs          *struct{}
	csi          *CSIVolumeSource
}

type CapacityCondition struct {
	capacity Capacity
}

func (c *CapacityCondition) Match(v *StructuredVolume) bool {
	return c.capacity.IsInRange(v.capacity)
}

type StorageClassCondition struct {
	storageClass []string
}

func (s *StorageClassCondition) Match(v *StructuredVolume) bool {
	if len(s.storageClass) == 0 {
		return true
	} else if v.storageClass == "" {
		return false
	} else {
		target := strings.TrimSpace(v.storageClass)
		for _, v := range s.storageClass {
			sc := strings.TrimSpace(v)
			if target == sc {
				return true
			}
		}
	}
	return false
}

type NFSCondition struct {
	nfs *struct{}
}

func (c *NFSCondition) Match(v *StructuredVolume) bool {
	if c.nfs == nil {
		return true
	} else if v.nfs != nil {
		return true
	}
	return false
}

type CSICondition struct {
	csi *CSIVolumeSource
}

func (c *CSICondition) Match(v *StructuredVolume) bool {
	if c.csi == nil {
		return true
	} else if v.csi != nil {
		return c.csi.driver == v.csi.driver
	}
	return false
}

func (p *PolicyConditionsMatcher) Match(v *StructuredVolume) string {
	for _, policy := range p.policies {
		for _, con := range policy.volConditions {
			isMatch := con.Match(v)
			if isMatch {
				return policy.action.Type
			}
		}
	}
	return ""
}

func (p *PolicyConditionsMatcher) addPolicy(vp *VolumePolicy) error {
	con, err := UnmarshalVolConditions(vp.Conditions)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshl volume conditions")
	}

	volCap, err := ParseCapacity(con.Capacity)
	if err != nil {
		return errors.Wrapf(err, "failed to parse condition capacity %s", con.Capacity)
	}
	newPolicy := &VolumePolicier{}
	newPolicy.action = vp.Action
	newPolicy.volConditions = append(newPolicy.volConditions, &CapacityCondition{capacity: *volCap})
	newPolicy.volConditions = append(newPolicy.volConditions, &StorageClassCondition{storageClass: con.StorageClass})
	newPolicy.volConditions = append(newPolicy.volConditions, &NFSCondition{nfs: con.NFS})
	newPolicy.volConditions = append(newPolicy.volConditions, &CSICondition{csi: con.CSI})
	p.policies = append(p.policies, newPolicy)
	return nil
}

// ParseCapacity parse string into capacity format
func ParseCapacity(capacity string) (*Capacity, error) {
	capacities := strings.Split(capacity, ",")
	var quantities []resource.Quantity
	if len(capacities) != 2 {
		return nil, fmt.Errorf("wrong format of Capacity %v", capacity)
	} else {
		for _, v := range capacities {
			if strings.TrimSpace(v) == "" {
				// case similar "10Gi,"
				// if empty, the quantity will assigned with 0
				quantities = append(quantities, *resource.NewQuantity(int64(0), resource.DecimalSI))
			} else {
				if quantity, err := resource.ParseQuantity(strings.TrimSpace(v)); err != nil {
					return nil, fmt.Errorf("wrong format of Capacity %v with err %v", v, err)
				} else {
					quantities = append(quantities, quantity)
				}
			}
		}
	}
	return &Capacity{lower: quantities[0], upper: quantities[1]}, nil
}

// IsInRange returns true if the quantity y is in range of capacity, or it returns false
func (c *Capacity) IsInRange(y resource.Quantity) bool {
	if c.lower.IsZero() && c.upper.Cmp(y) >= 0 {
		// [0, a] y
		return true
	} else if c.upper.IsZero() && c.lower.Cmp(y) <= 0 {
		// [b, 0] y
		return true
	} else if !c.lower.IsZero() && !c.upper.IsZero() {
		// [a, b] y
		return c.lower.Cmp(y) >= 0 && c.upper.Cmp(y) <= 0
	}
	return false
}

// UnmarshalVolConditions parse map[string]interface{} into VolumeConditions format
// and validate key fields of the map.
func UnmarshalVolConditions(con map[string]interface{}) (*VolumeConditions, error) {
	var volConditons *VolumeConditions
	buffer := new(bytes.Buffer)
	err := yaml.NewEncoder(buffer).Encode(con)
	if err != nil {
		return volConditons, fmt.Errorf("failed to encode volume conditions with err %v", err)
	}

	if err := decodeStruct(buffer, volConditons); err != nil {
		return nil, fmt.Errorf("failed to decode volume conditions with err %v", err)
	}
	return volConditons, nil
}
