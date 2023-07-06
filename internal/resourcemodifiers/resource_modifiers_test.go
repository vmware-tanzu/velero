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
			"sub.yml": "# kubectl delete cm jsonsub-configmap -n velero\n# kubectl create cm jsonsub-configmap --from-file /home/anshulahuja/workspace/fork-velero/tilt-resources/examples/sub.yml -nvelero\nversion: v1\nresourceModifierRules:\n- conditions:\n    groupKind: persistentvolumeclaims.storage.k8s.io\n    resourceNameRegex: \".*\"\n    namespaces:\n    - bar\n    - foo\n  patches:\n  - operation: replace\n    path: \"/spec/storageClassName\"\n    Value: \"premium\"\n  - operation: remove\n    path: \"/metadata/labels/test\"\n\n\n",
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
					GroupKind:         "persistentvolumeclaims.storage.k8s.io",
					ResourceNameRegex: ".*",
					Namespaces:        []string{"bar", "foo"},
				},
				Patches: []JsonPatch{
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
			"kind": "PersistentVolumeClaim",
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
			"kind": "PersistentVolumeClaim",
			"metadata": map[string]interface{}{
				"name":      "test-pvc",
				"namespace": "foo",
			},
			"spec": map[string]interface{}{
				"storageClassName": "premium",
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
		want    []error
		wantObj *unstructured.Unstructured
	}{
		{
			name: "test1",
			fields: fields{
				Version: "v1",
				ResourceModifierRules: []ResourceModifierRule{
					{
						Conditions: Conditions{
							GroupKind:         "persistentvolumeclaims",
							ResourceNameRegex: ".*",
							Namespaces:        []string{"foo"},
						},
						Patches: []JsonPatch{
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
			want:    nil,
			wantObj: pvcPremiumSc.DeepCopy(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &ResourceModifiers{
				Version:               tt.fields.Version,
				ResourceModifierRules: tt.fields.ResourceModifierRules,
			}
			got := p.ApplyResourceModifierRules(tt.args.obj, tt.args.groupResource, logrus.New())

			assert.Equal(t, len(tt.want), len(got))
			assert.Equal(t, *tt.wantObj, *tt.args.obj)
		})
	}
}
