package resourcemodifiers

import (
	"reflect"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGetResourceModifiersFromConfig(t *testing.T) {
	cm1 := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"sub.yml": "version: v1\nresourceModifierRules:\n- conditions:\n    groupKind: persistentvolumeclaims\n    resourceNameRegex: \".*\"\n    namespaces:\n    - bar\n    - foo\n  patches:\n  - operation: replace\n    path: \"/spec/storageClassName\"\n    value: \"premium\"\n  - operation: remove\n    path: \"/metadata/labels/test\"\n\n\n",
		},
	}

	rules1 := &ResourceModifiers{
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
	cm2 := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"sub.yml": "version: v1\nresourceModifierRules:\n- conditions:\n    groupKind: deployments.apps\n    resourceNameRegex: \"^test-.*$\"\n    namespaces:\n    - bar\n    - foo\n  patches:\n  - operation: add\n    path: \"/spec/template/spec/containers/0\"\n    value: \"{\\\"name\\\": \\\"nginx\\\", \\\"image\\\": \\\"nginx:1.14.2\\\", \\\"ports\\\": [{\\\"containerPort\\\": 80}]}\"\n  - operation: copy\n    from: \"/spec/template/spec/containers/0\"\n    path: \"/spec/template/spec/containers/1\"\n\n\n",
		},
	}

	rules2 := &ResourceModifiers{
		Version: "v1",
		ResourceModifierRules: []ResourceModifierRule{
			{
				Conditions: Conditions{
					GroupKind:         "deployments.apps",
					ResourceNameRegex: "^test-.*$",
					Namespaces:        []string{"bar", "foo"},
				},
				Patches: []JSONPatch{
					{
						Operation: "add",
						Path:      "/spec/template/spec/containers/0",
						Value:     `{"name": "nginx", "image": "nginx:1.14.2", "ports": [{"containerPort": 80}]}`,
					},
					{
						Operation: "copy",
						From:      "/spec/template/spec/containers/0",
						Path:      "/spec/template/spec/containers/1",
					},
				},
			},
		},
	}

	cm3 := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"sub.yml": "version1: v1\nresourceModifierRules:\n- conditions:\n    groupKind: deployments.apps\n    resourceNameRegex: \"^test-.*$\"\n    namespaces:\n    - bar\n    - foo\n  patches:\n  - operation: add\n    path: \"/spec/template/spec/containers/0\"\n    value: \"{\\\"name\\\": \\\"nginx\\\", \\\"image\\\": \\\"nginx:1.14.2\\\", \\\"ports\\\": [{\\\"containerPort\\\": 80}]}\"\n  - operation: copy\n    from: \"/spec/template/spec/containers/0\"\n    path: \"/spec/template/spec/containers/1\"\n\n\n",
		},
	}

	type args struct {
		cm *v1.ConfigMap
	}
	tests := []struct {
		name    string
		args    args
		want    *ResourceModifiers
		wantErr bool
	}{
		{
			name: "test 1",
			args: args{
				cm: cm1,
			},
			want:    rules1,
			wantErr: false,
		},
		{
			name: "complex payload in add and copy operator",
			args: args{
				cm: cm2,
			},
			want:    rules2,
			wantErr: false,
		},
		{
			name: "invalid payload version1",
			args: args{
				cm: cm3,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "nil configmap",
			args: args{
				cm: nil,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetResourceModifiersFromConfig(tt.args.cm)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetResourceModifiersFromConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetResourceModifiersFromConfig() = %v, want %v", got, tt.want)
			}
		})
	}
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
				"replicas": "1",
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "nginx",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "nginx",
								"image": "nginx:latest",
							},
						},
					},
				},
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
				"replicas": "2",
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "nginx",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "nginx",
								"image": "nginx:latest",
							},
						},
					},
				},
			},
		},
	}
	deployNginxMysql := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "foo",
			},
			"spec": map[string]interface{}{
				"replicas": "1",
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "nginx",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "nginx",
								"image": "nginx:latest",
							},
							map[string]interface{}{
								"name":  "mysql",
								"image": "mysql:latest",
							},
						},
					},
				},
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
			name: "Invalid Regex throws error",
			fields: fields{
				Version: "v1",
				ResourceModifierRules: []ResourceModifierRule{
					{
						Conditions: Conditions{
							GroupKind:         "persistentvolumeclaims",
							ResourceNameRegex: "[a-z",
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
			wantErr: true,
			wantObj: pvcStandardSc.DeepCopy(),
		},
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
							ResourceNameRegex: "^test-.*$",
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
			name: "nginx deployment: test operator fails, skips substitution, no error",
			fields: fields{
				Version: "v1",
				ResourceModifierRules: []ResourceModifierRule{
					{
						Conditions: Conditions{
							GroupKind:         "deployments.apps",
							ResourceNameRegex: "^test-.*$",
							Namespaces:        []string{"foo"},
						},
						Patches: []JSONPatch{
							{
								Operation: "test",
								Path:      "/spec/replicas",
								Value:     "5",
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
		{
			name: "add container mysql to deployment",
			fields: fields{
				Version: "v1",
				ResourceModifierRules: []ResourceModifierRule{
					{
						Conditions: Conditions{
							GroupKind:         "deployments.apps",
							ResourceNameRegex: "^test-.*$",
							Namespaces:        []string{"foo"},
						},
						Patches: []JSONPatch{
							{
								Operation: "add",
								Path:      "/spec/template/spec/containers/1",
								Value:     `{"name": "mysql", "image": "mysql:latest"}`,
							},
						},
					},
				},
			},
			args: args{
				obj:           deployNginxOneReplica,
				groupResource: "deployments.apps",
			},
			wantErr: false,
			wantObj: deployNginxMysql,
		},
		{
			name: "Copy container 0 to container 1 and then modify container 1",
			fields: fields{
				Version: "v1",
				ResourceModifierRules: []ResourceModifierRule{
					{
						Conditions: Conditions{
							GroupKind:         "deployments.apps",
							ResourceNameRegex: "^test-.*$",
							Namespaces:        []string{"foo"},
						},
						Patches: []JSONPatch{
							{
								Operation: "copy",
								From:      "/spec/template/spec/containers/0",
								Path:      "/spec/template/spec/containers/1",
							},
							{
								Operation: "test",
								Path:      "/spec/template/spec/containers/1/image",
								Value:     "nginx:latest",
							},
							{
								Operation: "replace",
								Path:      "/spec/template/spec/containers/1/name",
								Value:     "mysql",
							},
							{
								Operation: "replace",
								Path:      "/spec/template/spec/containers/1/image",
								Value:     "mysql:latest",
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
			wantObj: deployNginxMysql.DeepCopy(),
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

func TestJSONPatch_ToString(t *testing.T) {
	type fields struct {
		Operation string
		From      string
		Path      string
		Value     string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "test",
			fields: fields{
				Operation: "test",
				Path:      "/spec/replicas",
				Value:     "1",
			},
			want: `{"op": "test", "from": "", "path": "/spec/replicas", "value": "1"}`,
		},
		{
			name: "replace",
			fields: fields{
				Operation: "replace",
				Path:      "/spec/replicas",
				Value:     "2",
			},
			want: `{"op": "replace", "from": "", "path": "/spec/replicas", "value": "2"}`,
		},
		{
			name: "add complex interfaces",
			fields: fields{
				Operation: "add",
				Path:      "/spec/template/spec/containers/0",
				Value:     `{"name": "nginx", "image": "nginx:1.14.2", "ports": [{"containerPort": 80}]}`,
			},
			want: `{"op": "add", "from": "", "path": "/spec/template/spec/containers/0", "value": {"name": "nginx", "image": "nginx:1.14.2", "ports": [{"containerPort": 80}]}}`,
		},
		{
			name: "remove",
			fields: fields{
				Operation: "remove",
				Path:      "/spec/template/spec/containers/0",
			},
			want: `{"op": "remove", "from": "", "path": "/spec/template/spec/containers/0", "value": ""}`,
		},
		{
			name: "move",
			fields: fields{
				Operation: "move",
				From:      "/spec/template/spec/containers/0",
				Path:      "/spec/template/spec/containers/1",
			},
			want: `{"op": "move", "from": "/spec/template/spec/containers/0", "path": "/spec/template/spec/containers/1", "value": ""}`,
		},
		{
			name: "copy",
			fields: fields{
				Operation: "copy",
				From:      "/spec/template/spec/containers/0",
				Path:      "/spec/template/spec/containers/1",
			},
			want: `{"op": "copy", "from": "/spec/template/spec/containers/0", "path": "/spec/template/spec/containers/1", "value": ""}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &JSONPatch{
				Operation: tt.fields.Operation,
				From:      tt.fields.From,
				Path:      tt.fields.Path,
				Value:     tt.fields.Value,
			}
			if got := p.ToString(); got != tt.want {
				t.Errorf("JSONPatch.ToString() = %v, want %v", got, tt.want)
			}
		})
	}
}
