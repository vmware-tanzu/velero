package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

/*

- A VolumeSnapshotProvider is one per block storage provider. Since each PV type
  will map to at most one VolumeSnapshotProvider, there's no concept of a "default"
  VolumeSnapshotProvider. There is, however, a default *target* within each provider.

*/

type VolumeSnapshotProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   VolumeSnapshotProviderSpec   `json:"spec"`
	Status VolumeSnapshotProviderStatus `json:"status,omitempty"`
}

type VolumeSnapshotProviderSpec struct {
	// Locations is a list of the snapshot storage locations
	// for this provider
	Locations []VolumeSnapshotLocation `json:"locations"`

	// DefaultLocationName is the name of the default storage
	// location for this provider
	DefaultLocationName string `json:"defaultLocationName"`
}

type VolumeSnapshotLocation struct {
	Name   string            `json:"name"`
	Config map[string]string `json:"config"`
}

type VolumeSnapshotProviderStatus struct {
}

// NewVolumeSnapshotProvider is an example of what constructing a
// VolumeSnapshotProvider looks like. To be removed.
func NewVolumeSnapshotProvider() *VolumeSnapshotProvider {
	return &VolumeSnapshotProvider{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: DefaultNamespace,
			Name:      "aws-ebs",
		},
		Spec: VolumeSnapshotProviderSpec{
			Locations: []VolumeSnapshotLocation{
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
			DefaultLocationName: "us-east-1",
		},
	}
}
