package resourcemodifiers

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGetResourceModifiersFromConfig(t *testing.T) {
	// Create a test ConfigMap
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"sub.yml": "version: v1\nresourceModifierRules:\n- conditions:\n    groupKind: persistentvolumeclaims\n    resourceNameRegex: \".*\"\n    namespaces:\n    - bar\n    - foo\n  patches:\n  - operation: replace\n    path: \"/spec/storageClassName\"\n    value: \"premium\"\n  - operation: remove\n    path: \"/metadata/labels/test\"\n\n\n",
		},
	}

	// Call the function and check for errors
	original, err := GetResourceModifiersFromConfig(cm)
	assert.Nil(t, err)

	// Check that the returned resourceModifiers object contains the expected data
	assert.Equal(t, "v1", original.Version)
	assert.Len(t, original.ResourceModifierRules, 1)
	assert.Len(t, original.ResourceModifierRules[0].Patches, 2)

	expected := &ResourceModifiers{
		Version: "v1",
		ResourceModifierRules: []ResourceModifierRule{
			{
				Conditions: Conditions{
					GroupKind:         "persistentvolumeclaims",
					ResourceNameRegex: ".*",
					Namespaces:        []string{"bar", "foo"},
				},
				Patches: []JSONPatch{
					{
						Operation: "replace",
						Path:      "/spec/storageClassName",
						Value:     "premium",
					},
					{
						Operation: "remove",
						Path:      "/metadata/labels/test",
					},
				},
			},
		},
	}
	if err != nil {
		t.Fatalf("failed to build policy with error %v", err)
	}
	assert.Equal(t, original, expected)
}

func TestResourceModifiers_ApplyResourceModifierRules(t *testing.T) {

	pvcStandardSc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "PersistentVolumeClaim",
			"metadata": map[string]interface{}{
				"name":      "test-pvc",
				"namespace": "foo",
			},
			"spec": map[string]interface{}{
				"storageClassName": "standard",
			},
		},
	}

	pvcPremiumSc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "PersistentVolumeClaim",
			"metadata": map[string]interface{}{
				"name":      "test-pvc",
				"namespace": "foo",
			},
			"spec": map[string]interface{}{
				"storageClassName": "premium",
			},
		},
	}

	deployNginxOneReplica := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "foo",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "nginx",
						"image": "nginx:latest",
					},
				},
				"replicas": "1",
			},
		},
	}
	deployNginxTwoReplica := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "foo",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "nginx",
						"image": "nginx:latest",
					},
				},
				"replicas": "2",
			},
		},
	}

	type fields struct {
		Version               string
		ResourceModifierRules []ResourceModifierRule
	}
	type args struct {
		obj           *unstructured.Unstructured
		groupResource string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		wantObj *unstructured.Unstructured
	}{
		{
			name: "pvc with standard storage class should be patched to premium",
			fields: fields{
				Version: "v1",
				ResourceModifierRules: []ResourceModifierRule{
					{
						Conditions: Conditions{
							GroupKind:         "persistentvolumeclaims",
							ResourceNameRegex: ".*",
							Namespaces:        []string{"foo"},
						},
						Patches: []JSONPatch{
							{
								Operation: "test",
								Path:      "/spec/storageClassName",
								Value:     "standard",
							},
							{
								Operation: "replace",
								Path:      "/spec/storageClassName",
								Value:     "premium",
							},
						},
					},
				},
			},
			args: args{
				obj:           pvcStandardSc.DeepCopy(),
				groupResource: "persistentvolumeclaims",
			},
			wantErr: false,
			wantObj: pvcPremiumSc.DeepCopy(),
		},
		{
			name: "nginx deployment: 1 -> 2 replicas",
			fields: fields{
				Version: "v1",
				ResourceModifierRules: []ResourceModifierRule{
					{
						Conditions: Conditions{
							GroupKind:         "deployments.apps",
							ResourceNameRegex: "test-.*",
							Namespaces:        []string{"foo"},
						},
						Patches: []JSONPatch{
							{
								Operation: "test",
								Path:      "/spec/replicas",
								Value:     "1",
							},
							{
								Operation: "replace",
								Path:      "/spec/replicas",
								Value:     "2",
							},
						},
					},
				},
			},
			args: args{
				obj:           deployNginxOneReplica.DeepCopy(),
				groupResource: "deployments.apps",
			},
			wantErr: false,
			wantObj: deployNginxTwoReplica.DeepCopy(),
		},
		{
			name: "nginx deployment: Empty Resource Regex",
			fields: fields{
				Version: "v1",
				ResourceModifierRules: []ResourceModifierRule{
					{
						Conditions: Conditions{
							GroupKind:  "deployments.apps",
							Namespaces: []string{"foo"},
						},
						Patches: []JSONPatch{
							{
								Operation: "test",
								Path:      "/spec/replicas",
								Value:     "1",
							},
							{
								Operation: "replace",
								Path:      "/spec/replicas",
								Value:     "2",
							},
						},
					},
				},
			},
			args: args{
				obj:           deployNginxOneReplica.DeepCopy(),
				groupResource: "deployments.apps",
			},
			wantErr: false,
			wantObj: deployNginxTwoReplica.DeepCopy(),
		},
		{
			name: "nginx deployment: Empty Resource Regex  and namespaces list",
			fields: fields{
				Version: "v1",
				ResourceModifierRules: []ResourceModifierRule{
					{
						Conditions: Conditions{
							GroupKind: "deployments.apps",
						},
						Patches: []JSONPatch{
							{
								Operation: "test",
								Path:      "/spec/replicas",
								Value:     "1",
							},
							{
								Operation: "replace",
								Path:      "/spec/replicas",
								Value:     "2",
							},
						},
					},
				},
			},
			args: args{
				obj:           deployNginxOneReplica.DeepCopy(),
				groupResource: "deployments.apps",
			},
			wantErr: false,
			wantObj: deployNginxTwoReplica.DeepCopy(),
		},
		{
			name: "nginx deployment: namespace doesn't match",
			fields: fields{
				Version: "v1",
				ResourceModifierRules: []ResourceModifierRule{
					{
						Conditions: Conditions{
							GroupKind:         "deployments.apps",
							ResourceNameRegex: ".*",
							Namespaces:        []string{"bar"},
						},
						Patches: []JSONPatch{
							{
								Operation: "test",
								Path:      "/spec/replicas",
								Value:     "1",
							},
							{
								Operation: "replace",
								Path:      "/spec/replicas",
								Value:     "2",
							},
						},
					},
				},
			},
			args: args{
				obj:           deployNginxOneReplica.DeepCopy(),
				groupResource: "deployments.apps",
			},
			wantErr: false,
			wantObj: deployNginxOneReplica.DeepCopy(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &ResourceModifiers{
				Version:               tt.fields.Version,
				ResourceModifierRules: tt.fields.ResourceModifierRules,
			}
			got := p.ApplyResourceModifierRules(tt.args.obj, tt.args.groupResource, logrus.New())

			assert.Equal(t, tt.wantErr, len(got) > 0)
			assert.Equal(t, *tt.wantObj, *tt.args.obj)
		})
	}
}
