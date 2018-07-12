package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

/*

- A VolumeStorageProvider is one per block storage provider. Since each PV type
  will map to at most one VolumeStorageProvider, there's no concept of a "default"
  VolumeStorageProvider. There is, however, a default *target* within each provider.

*/

type VolumeStorageProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   VolumeStorageProviderSpec   `json:"spec"`
	Status VolumeStorageProviderStatus `json:"status,omitempty"`
}

type VolumeStorageProviderSpec struct {
	// Targets is a list of the storage targets
	// for this provider
	Targets []VolumeStorageTarget `json:"targets"`

	// DefaultTargetName is the name of the default storage
	// target for this provider
	DefaultTargetName string `json:"defaultTargetName"`
}

type VolumeStorageTarget struct {
	Name   string            `json:"name"`
	Config map[string]string `json:"config"`
}

type VolumeStorageProviderStatus struct {
}

// NewVolumeStorageProvider is an example of what constructing a
// VolumeStorageProvider looks like. To be removed.
func NewVolumeStorageProvider() *VolumeStorageProvider {
	return &VolumeStorageProvider{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: DefaultNamespace,
			Name:      "aws-ebs",
		},
		Spec: VolumeStorageProviderSpec{
			Targets: []VolumeStorageTarget{
				{
					Name: "us-east-1",
					Config: map[string]string{
						"region": "us-east-1",
					},
				},
				{
					Name: "us-west-1",
					Config: map[string]string{
						"region": "us-west-1",
					},
				},
			},
			DefaultTargetName: "us-east-1",
		},
	}
}
