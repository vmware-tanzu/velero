package restore

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestNewAdmissionWebhookConfigurationActionExecute(t *testing.T) {
	action := NewAdmissionWebhookConfigurationAction(velerotest.NewLogger())
	cases := []struct {
		name                    string
		itemJSON                string
		wantErr                 bool
		NoneSideEffectsIndex    []int // the indexes with sideEffects that arereset to None
		NotNoneSideEffectsIndex []int // the indexes with sideEffects that are not reset to None
	}{
		{
			name: "v1 mutatingwebhookconfiguration with sideEffects as Unknown",
			itemJSON: `{
				 "apiVersion": "admissionregistration.k8s.io/v1",
				 "kind": "MutatingWebhookConfiguration",
				 "metadata": {
					  "name": "my-test-mutating"
				 },
				 "webhooks": [
					  {
						  "clientConfig": {
							   "url": "https://mytest.org"
						  },
						  "rules": [
							   {
									"apiGroups": [
										""
									],
									"apiVersions": [
										"v1"
									],
									"operations": [
										"CREATE"
									],
									"resources": [
										"pods"
									],
									"scope": "Namespaced"
							   }
						  ],
						  "sideEffects": "Unknown"
					  }
				 ]
			}`,
			wantErr:              false,
			NoneSideEffectsIndex: []int{0},
		},
		{
			name: "v1 validatingwebhookconfiguration with sideEffects as Some",
			itemJSON: `{
				 "apiVersion": "admissionregistration.k8s.io/v1",
				 "kind": "ValidatingWebhookConfiguration",
				 "metadata": {
					  "name": "my-test-validating"
				 },
				 "webhooks": [
					  {
						  "clientConfig": {
							   "url": "https://mytest.org"
						  },
						  "rules": [
							   {
									"apiGroups": [
										""
									],
									"apiVersions": [
										"v1"
									],
									"operations": [
										"CREATE"
									],
									"resources": [
										"pods"
									],
									"scope": "Namespaced"
							   }
						  ],
						  "sideEffects": "Some"
					  }
				 ]
			}`,
			wantErr:              false,
			NoneSideEffectsIndex: []int{0},
		},
		{
			name: "v1beta1 validatingwebhookconfiguration with sideEffects as Some, nothing should change",
			itemJSON: `{
				 "apiVersion": "admissionregistration.k8s.io/v1beta1",
				 "kind": "ValidatingWebhookConfiguration",
				 "metadata": {
					  "name": "my-test-validating"
				 },
				 "webhooks": [
					  {
						  "clientConfig": {
							   "url": "https://mytest.org"
						  },
						  "rules": [
							   {
									"apiGroups": [
										""
									],
									"apiVersions": [
										"v1"
									],
									"operations": [
										"CREATE"
									],
									"resources": [
										"pods"
									],
									"scope": "Namespaced"
							   }
						  ],
						  "sideEffects": "Some"
					  }
				 ]
			}`,
			wantErr:                 false,
			NotNoneSideEffectsIndex: []int{0},
		},
		{
			name: "v1 validatingwebhookconfiguration with multiple invalid sideEffects",
			itemJSON: `{
				 "apiVersion": "admissionregistration.k8s.io/v1",
				 "kind": "ValidatingWebhookConfiguration",
				 "metadata": {
					  "name": "my-test-validating"
				 },
				 "webhooks": [
					  {
						  "clientConfig": {
							   "url": "https://mytest.org"
						  },
						  "sideEffects": "Some"
					  },
					  {
						  "clientConfig": {
							   "url": "https://mytest2.org"
						  },
						  "sideEffects": "Some"
					  }
				 ]
			}`,
			wantErr:              false,
			NoneSideEffectsIndex: []int{0, 1},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			o := map[string]interface{}{}
			json.Unmarshal([]byte(tt.itemJSON), &o)
			input := &velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{
					Object: o,
				},
			}
			output, err := action.Execute(input)
			if tt.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
			if tt.NoneSideEffectsIndex != nil {
				wb, _, err := unstructured.NestedSlice(output.UpdatedItem.UnstructuredContent(), "webhooks")
				assert.Nil(t, err)
				for _, i := range tt.NoneSideEffectsIndex {
					it, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&wb[i])
					assert.Nil(t, err)
					s := it["sideEffects"].(string)
					assert.Equal(t, "None", s)
				}
			}
			if tt.NotNoneSideEffectsIndex != nil {
				wb, _, err := unstructured.NestedSlice(output.UpdatedItem.UnstructuredContent(), "webhooks")
				assert.Nil(t, err)
				for _, i := range tt.NotNoneSideEffectsIndex {
					it, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&wb[i])
					assert.Nil(t, err)
					s := it["sideEffects"].(string)
					assert.NotEqual(t, "None", s)
				}
			}
		})
	}
}
