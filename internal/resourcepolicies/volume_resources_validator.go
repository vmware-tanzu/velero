package resourcepolicies

import (
	"fmt"
	"io"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

const currentSupportDataVersion = "v1"

type csiVolumeSource struct {
	Driver string `yaml:"driver,omitempty"`
}

type nFSVolumeSource struct {
	// Server is the hostname or IP address of the NFS server
	Server string `yaml:"server,omitempty"`
	// Path is the exported NFS share
	Path string `yaml:"path,omitempty"`
}

// volumeConditions defined the current format of conditions we parsed
type volumeConditions struct {
	Capacity     string           `yaml:"capacity,omitempty"`
	StorageClass []string         `yaml:"storageClass,omitempty"`
	NFS          *nFSVolumeSource `yaml:"nfs,omitempty"`
	CSI          *csiVolumeSource `yaml:"csi,omitempty"`
}

func (c *capacityCondition) validate() error {
	// [0, a]
	// [a, b]
	// [b, 0]
	// ==> low <= upper or upper is zero
	if (c.capacity.upper.Cmp(c.capacity.lower) >= 0) ||
		(!c.capacity.lower.IsZero() && c.capacity.upper.IsZero()) {
		return nil
	}
	return errors.Errorf("illegal values for capacity %v", c.capacity)

}

func (s *storageClassCondition) validate() error {
	// validate by yamlv3
	return nil
}

func (c *nfsCondition) validate() error {
	// validate by yamlv3
	return nil
}

func (c *csiCondition) validate() error {
	// validate by yamlv3
	return nil
}

// decodeStruct restric validate the keys in decoded mappings to exist as fields in the struct being decoded into
func decodeStruct(r io.Reader, s interface{}) error {
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	return dec.Decode(s)
}

// validate check action format
func (a *Action) validate() error {
	// validate Type
	if a.Type != Skip {
		return fmt.Errorf("invalid action type %s", a.Type)
	}

	// TODO validate parameters
	return nil
}
