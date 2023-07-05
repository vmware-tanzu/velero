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
			"sub.yml": "# kubectl delete cm jsonsub-configmap -n velero\n# kubectl create cm jsonsub-configmap --from-file /home/anshulahuja/workspace/fork-velero/tilt-resources/examples/sub.yml -nvelero\nversion: v1\nresourceModifierRules:\n- conditions:\n    groupKind: persistentvolumeclaims.storage.k8s.io\n    resourceNameRegex: \".*\"\n    namespaces:\n    - bar\n    - foo\n  patches:\n  - operation: replace\n    path: \"/spec/storageClassName\"\n    newValue: \"premium\"\n  - operation: remove\n    path: \"/metadata/labels/test\"\n\n\n",
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
						NewValue:  "premium",
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

// pvc1 := &unstructured.Unstructured{
// 	Object: map[string]interface{}{
// 		"kind": "PersistentVolumeClaim",
// 		"metadata": map[string]interface{}{
// 			"name":      "test-pvc",
// 			"namespace": "foo",
// 		},
// 	},
// }

func TestResourceModifiers_ApplyResourceModifierRules(t *testing.T) {
	type fields struct {
		Version               string
		ResourceModifierRules []ResourceModifierRule
	}
	type args struct {
		obj *unstructured.Unstructured
		log logrus.FieldLogger
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []error
		wantObj *unstructured.Unstructured
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &ResourceModifiers{
				Version:               tt.fields.Version,
				ResourceModifierRules: tt.fields.ResourceModifierRules,
			}
			// comapre lenght of got and want  {
			got := p.ApplyResourceModifierRules(tt.args.obj, tt.args.log)
			assert.Equal(t, len(got), len(tt.want))
			assert.Equal(t, *tt.args.obj, *tt.wantObj)
		})
	}
}
