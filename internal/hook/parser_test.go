package hook

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	discovery_mocks "github.com/vmware-tanzu/velero/pkg/discovery/mocks"
)

func Test_listApplicableResourceBackupHooks(t *testing.T) {
	discoveryHelper := discovery_mocks.NewHelper(t)
	parser := &parser{
		discoveryHelper: discoveryHelper,
	}
	log := logrus.New()
	res := &unstructured.Unstructured{}
	err := res.UnmarshalJSON([]byte(pod))
	require.NoError(t, err)
	annotations := map[string]string{
		"pre.hook.backup.velero.io/container":  "nginx",
		"pre.hook.backup.velero.io/command":    "command-for-pre",
		"pre.hook.backup.velero.io/on-error":   "Continue",
		"pre.hook.backup.velero.io/timeout":    "60s",
		"post.hook.backup.velero.io/container": "nginx",
		"post.hook.backup.velero.io/command":   "command-for-post",
		"post.hook.backup.velero.io/on-error":  "Fail",
		"post.hook.backup.velero.io/timeout":   "90s",
	}
	res.SetAnnotations(annotations)

	// not pod
	gr := schema.GroupResource{
		Group:    "",
		Resource: "services",
	}
	hookSpecs := []velerov1.BackupResourceHookSpec{}
	pre := true
	hooks, err := parser.listApplicableResourceBackupHooks(log, res, gr, hookSpecs, pre)
	require.NoError(t, err)
	assert.Empty(t, hooks)

	// pre hook defined in resource annotation
	gr = schema.GroupResource{
		Group:    "",
		Resource: "pods",
	}
	hookSpecs = []velerov1.BackupResourceHookSpec{}
	pre = true
	hooks, err = parser.listApplicableResourceBackupHooks(log, res, gr, hookSpecs, pre)
	require.NoError(t, err)
	require.Len(t, hooks, 1)
	assert.Equal(t, "<from-annotation>", hooks[0].Name)
	assert.Equal(t, TypePodBackupPreHook, hooks[0].Type)
	assert.Equal(t, 0, hooks[0].Index)
	assert.True(t, hooks[0].ContinueOnError)

	// post hook defined in resource annotation
	gr = schema.GroupResource{
		Group:    "",
		Resource: "pods",
	}
	hookSpecs = []velerov1.BackupResourceHookSpec{}
	pre = false
	hooks, err = parser.listApplicableResourceBackupHooks(log, res, gr, hookSpecs, pre)
	require.NoError(t, err)
	require.Len(t, hooks, 1)
	assert.Equal(t, "<from-annotation>", hooks[0].Name)
	assert.Equal(t, TypePodBackupPostHook, hooks[0].Type)
	assert.Equal(t, 0, hooks[0].Index)
	assert.False(t, hooks[0].ContinueOnError)

	// hook defined in backup spec
	res.SetAnnotations(nil) // remove the hooks defined in annotations
	gr = schema.GroupResource{
		Group:    "",
		Resource: "pods",
	}
	hookSpecs = []velerov1.BackupResourceHookSpec{
		{
			Name: "hook01",
			PreHooks: []velerov1.BackupResourceHook{
				{
					Exec: &velerov1api.ExecHook{
						Container: "nginx",
						Command:   []string{"command01"},
						OnError:   velerov1.HookErrorModeContinue,
						Timeout: metav1.Duration{
							Duration: 60 * time.Second,
						},
					},
				},
				{
					Exec: &velerov1api.ExecHook{
						Container: "nginx",
						Command:   []string{"command02"},
						OnError:   velerov1.HookErrorModeContinue,
						Timeout: metav1.Duration{
							Duration: 60 * time.Second,
						},
					},
				},
			},
		},
		{
			Name: "hook02",
			PreHooks: []velerov1.BackupResourceHook{
				{
					Exec: &velerov1api.ExecHook{
						Container: "nginx",
						Command:   []string{"command03"},
						OnError:   velerov1.HookErrorModeFail,
						Timeout: metav1.Duration{
							Duration: 90 * time.Second,
						},
					},
				},
			},
		},
	}
	pre = true
	hooks, err = parser.listApplicableResourceBackupHooks(log, res, gr, hookSpecs, pre)
	require.NoError(t, err)
	require.Len(t, hooks, 3)
	assert.Equal(t, "hook01", hooks[0].Name)
	assert.Equal(t, TypePodBackupPreHook, hooks[0].Type)
	assert.Equal(t, 0, hooks[0].Index)
	assert.True(t, hooks[0].ContinueOnError)
	assert.Equal(t, "hook01", hooks[1].Name)
	assert.Equal(t, TypePodBackupPreHook, hooks[1].Type)
	assert.Equal(t, 1, hooks[1].Index)
	assert.True(t, hooks[1].ContinueOnError)
	assert.Equal(t, "hook02", hooks[2].Name)
	assert.Equal(t, TypePodBackupPreHook, hooks[2].Type)
	assert.Equal(t, 0, hooks[2].Index)
	assert.False(t, hooks[2].ContinueOnError)
}

func Test_assembleExecHook(t *testing.T) {
	log := logrus.New()
	container := "container01"
	command := "cmd"

	// invalid "timeout" and "onError"
	timeout := "invalid"
	onError := "invalid"
	hook := assembleExecHook(log, container, command, timeout, onError)
	assert.Equal(t, container, hook.Container)
	assert.Equal(t, []string{"cmd"}, hook.Command)
	assert.Equal(t, defaultExecTimeout, hook.Timeout.Duration)
	assert.Equal(t, "", string(hook.OnError))

	// valid "timeout" and "onError"
	timeout = "60s"
	onError = "Continue"
	hook = assembleExecHook(log, container, command, timeout, onError)
	assert.Equal(t, container, hook.Container)
	assert.Equal(t, []string{"cmd"}, hook.Command)
	assert.Equal(t, 60*time.Second, hook.Timeout.Duration)
	assert.Equal(t, velerov1api.HookErrorModeContinue, hook.OnError)
}

func Test_matchesResource(t *testing.T) {
	res := &unstructured.Unstructured{}
	err := res.UnmarshalJSON([]byte(pod))
	require.NoError(t, err)

	discoveryHelper := discovery_mocks.NewHelper(t)

	gr := schema.GroupResource{
		Group:    "",
		Resource: "pods",
	}

	// check namespace
	includedNamespaces := []string{"not-matched-namespace"}
	excludedNamespaces := []string{}
	includedResources := []string{}
	excludedResources := []string{}
	labelSelector := &metav1.LabelSelector{}
	match, err := matchesResource(res, gr, includedNamespaces, excludedNamespaces, includedResources, excludedResources, labelSelector, discoveryHelper)
	require.NoError(t, err)
	assert.False(t, match)

	// check resource
	includedNamespaces = []string{"nginx"}
	excludedNamespaces = []string{}
	includedResources = []string{"services"}
	excludedResources = []string{}
	labelSelector = &metav1.LabelSelector{}
	discoveryHelper.On("ResourceFor", mock.Anything).Return(schema.GroupVersionResource{Version: "v1", Resource: "services"}, metav1.APIResource{}, nil)
	match, err = matchesResource(res, gr, includedNamespaces, excludedNamespaces, includedResources, excludedResources, labelSelector, discoveryHelper)
	require.NoError(t, err)
	assert.False(t, match)

	// check label selector: invalid label selector
	includedNamespaces = []string{"nginx"}
	excludedNamespaces = []string{}
	includedResources = []string{"pods"}
	excludedResources = []string{}
	labelSelector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": "not-matched-label",
		},
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "app",
				Operator: "nonexistent-operator",
				Values:   []string{"not-matched-label"},
			},
		},
	}
	discoveryHelper.On("ResourceFor").Unset()
	discoveryHelper.On("ResourceFor", mock.Anything).Return(schema.GroupVersionResource{Version: "v1", Resource: "pods"}, metav1.APIResource{}, nil)
	_, err = matchesResource(res, gr, includedNamespaces, excludedNamespaces, includedResources, excludedResources, labelSelector, discoveryHelper)
	require.Error(t, err)

	// check label selector: valid label selector
	includedNamespaces = []string{"nginx"}
	excludedNamespaces = []string{}
	includedResources = []string{"pods"}
	excludedResources = []string{}
	labelSelector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": "not-matched-label",
		},
	}
	discoveryHelper.On("ResourceFor").Unset()
	discoveryHelper.On("ResourceFor", mock.Anything).Return(schema.GroupVersionResource{Version: "v1", Resource: "pods"}, metav1.APIResource{}, nil)
	match, err = matchesResource(res, gr, includedNamespaces, excludedNamespaces, includedResources, excludedResources, labelSelector, discoveryHelper)
	require.NoError(t, err)
	assert.False(t, match)

	labelSelector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app": "nginx",
		},
	}
	match, err = matchesResource(res, gr, includedNamespaces, excludedNamespaces, includedResources, excludedResources, labelSelector, discoveryHelper)
	require.NoError(t, err)
	assert.True(t, match)
}
