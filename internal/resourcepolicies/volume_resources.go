package resourcepolicies

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type volumePolicier struct {
	action        Action
	volConditions []volumeCondition
}

type volumeCondition interface {
	Validate() (bool, error)
	Match(v *StructuredVolume) bool
}

type policyConditionsMatcher struct {
	policies []*volumePolicier
}

// Capacity consist of the lower and upper boundary
type Capacity struct {
	lower resource.Quantity
	upper resource.Quantity
}

type StructuredVolume struct {
	capacity     resource.Quantity
	storageClass string
	nfs          *nFSVolumeSource
	csi          *csiVolumeSource
}

func (s *StructuredVolume) ParsePV(pv *corev1api.PersistentVolume) {
	s.capacity = *pv.Spec.Capacity.Storage()
	s.storageClass = pv.Spec.StorageClassName
	nfs := pv.Spec.NFS
	if nfs != nil {
		s.nfs = &nFSVolumeSource{Server: nfs.Server, Path: nfs.Path}
	}

	csi := pv.Spec.CSI
	if csi != nil {
		s.csi = &csiVolumeSource{Driver: csi.Driver}
	}
}

func (s *StructuredVolume) ParsePodVolume(vol *corev1api.Volume) {
	nfs := vol.NFS
	if nfs != nil {
		s.nfs = &nFSVolumeSource{Server: nfs.Server, Path: nfs.Path}
	}

	csi := vol.CSI
	if csi != nil {
		s.csi = &csiVolumeSource{Driver: csi.Driver}
	}
}

type capacityCondition struct {
	capacity Capacity
}

func (c *capacityCondition) Match(v *StructuredVolume) bool {
	return c.capacity.isInRange(v.capacity)
}

type storageClassCondition struct {
	storageClass []string
}

func (s *storageClassCondition) Match(v *StructuredVolume) bool {
	if len(s.storageClass) == 0 {
		return true
	}

	if v.storageClass == "" {
		return false
	}

	for _, sc := range s.storageClass {
		if v.storageClass == sc {
			return true
		}
	}

	return false
}

type nfsCondition struct {
	nfs *nFSVolumeSource
}

func (c *nfsCondition) Match(v *StructuredVolume) bool {
	if c.nfs == nil {
		return true
	}
	if v.nfs == nil {
		return false
	}

	if c.nfs.Path == "" {
		if c.nfs.Server == "" {
			return true
		}
		if c.nfs.Server != v.nfs.Server {
			return false
		}
		return true
	}
	if c.nfs.Path != v.nfs.Path {
		return false
	}
	if c.nfs.Server == "" {
		return true
	}
	if c.nfs.Server != v.nfs.Server {
		return false
	}
	return true

}

type csiCondition struct {
	csi *csiVolumeSource
}

func (c *csiCondition) Match(v *StructuredVolume) bool {
	if c.csi == nil {
		return true
	}

	if v.csi == nil {
		return false
	}

	return c.csi.Driver == v.csi.Driver
}

func (p *policyConditionsMatcher) Match(v *StructuredVolume) *Action {
	for _, policy := range p.policies {
		isAllMatch := false
		for _, con := range policy.volConditions {
			if !con.Match(v) {
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

func (p *policyConditionsMatcher) addPolicy(vp *VolumePolicy) error {
	con, err := unmarshalVolConditions(vp.Conditions)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshl volume conditions")
	}

	volCap, err := parseCapacity(con.Capacity)
	if err != nil {
		return errors.Wrapf(err, "failed to parse condition capacity %s", con.Capacity)
	}
	newPolicy := &volumePolicier{}
	newPolicy.action = vp.Action
	newPolicy.volConditions = append(newPolicy.volConditions, &capacityCondition{capacity: *volCap})
	newPolicy.volConditions = append(newPolicy.volConditions, &storageClassCondition{storageClass: con.StorageClass})
	newPolicy.volConditions = append(newPolicy.volConditions, &nfsCondition{nfs: con.NFS})
	newPolicy.volConditions = append(newPolicy.volConditions, &csiCondition{csi: con.CSI})
	p.policies = append(p.policies, newPolicy)
	return nil
}

// parseCapacity parse string into capacity format
func parseCapacity(capacity string) (*Capacity, error) {
	if capacity == "" {
		capacity = ","
	}
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

// isInRange returns true if the quantity y is in range of capacity, or it returns false
func (c *Capacity) isInRange(y resource.Quantity) bool {
	if c.lower.IsZero() && c.upper.Cmp(y) >= 0 {
		// [0, a] y
		return true
	}
	if c.upper.IsZero() && c.lower.Cmp(y) <= 0 {
		// [b, 0] y
		return true
	}
	if !c.lower.IsZero() && !c.upper.IsZero() {
		// [a, b] y
		return c.lower.Cmp(y) <= 0 && c.upper.Cmp(y) >= 0
	}
	return false
}

// unmarshalVolConditions parse map[string]interface{} into volumeConditions format
// and validate key fields of the map.
func unmarshalVolConditions(con map[string]interface{}) (*volumeConditions, error) {
	volConditons := &volumeConditions{}
	buffer := new(bytes.Buffer)
	err := yaml.NewEncoder(buffer).Encode(con)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode volume conditions")
	}

	if err := decodeStruct(buffer, volConditons); err != nil {
		return nil, errors.Wrap(err, "failed to decode volume conditions")
	}
	return volConditons, nil
}
