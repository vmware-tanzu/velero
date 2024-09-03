package install

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	goruntime "runtime"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1crds "github.com/vmware-tanzu/velero/config/crd/v1/crds"
	"github.com/vmware-tanzu/velero/pkg/test"
)

func TestInstall(t *testing.T) {
	dc := &test.FakeDynamicClient{}
	dc.On("Create", mock.Anything).Return(&unstructured.Unstructured{}, nil)

	factory := &test.FakeDynamicFactory{}
	factory.On("ClientForGroupVersionResource", mock.Anything, mock.Anything, mock.Anything).Return(dc, nil)

	c := fake.NewClientBuilder().WithObjects(
		&apiextv1.CustomResourceDefinition{
			ObjectMeta: v1.ObjectMeta{
				Name: "backuprepositories.velero.io",
			},

			Status: apiextv1.CustomResourceDefinitionStatus{
				Conditions: []apiextv1.CustomResourceDefinitionCondition{
					{
						Type:   apiextv1.Established,
						Status: apiextv1.ConditionTrue,
					},
					{
						Type:   apiextv1.NamesAccepted,
						Status: apiextv1.ConditionTrue,
					},
				},
			},
		},
	).Build()

	resources := &unstructured.UnstructuredList{}
	require.NoError(t, appendUnstructured(resources, v1crds.CRDs[0]))
	require.NoError(t, appendUnstructured(resources, Namespace("velero")))

	assert.NoError(t, Install(factory, c, resources, os.Stdout))
}

func Test_crdsAreReady(t *testing.T) {
	c := fake.NewClientBuilder().WithObjects(
		&apiextv1beta1.CustomResourceDefinition{
			ObjectMeta: v1.ObjectMeta{
				Name: "backuprepositories.velero.io",
			},

			Status: apiextv1beta1.CustomResourceDefinitionStatus{
				Conditions: []apiextv1beta1.CustomResourceDefinitionCondition{
					{
						Type:   apiextv1beta1.Established,
						Status: apiextv1beta1.ConditionTrue,
					},
					{
						Type:   apiextv1beta1.NamesAccepted,
						Status: apiextv1beta1.ConditionTrue,
					},
				},
			},
		},
	).Build()

	crd := &apiextv1beta1.CustomResourceDefinition{
		TypeMeta: v1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: "v1beta1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "backuprepositories.velero.io",
		},
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(crd)
	require.NoError(t, err)

	crds := []*unstructured.Unstructured{
		{
			Object: obj,
		},
	}

	ready, err := crdsAreReady(c, crds)
	require.NoError(t, err)
	assert.True(t, ready)
}

func TestDeploymentIsReady(t *testing.T) {
	deployment := &appsv1.Deployment{
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:               appsv1.DeploymentAvailable,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: v1.NewTime(time.Now().Add(-15 * time.Second)),
				},
			},
		},
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(deployment)
	require.NoError(t, err)

	dc := &test.FakeDynamicClient{}
	dc.On("Get", mock.Anything, mock.Anything).Return(&unstructured.Unstructured{Object: obj}, nil)

	factory := &test.FakeDynamicFactory{}
	factory.On("ClientForGroupVersionResource", mock.Anything, mock.Anything, mock.Anything).Return(dc, nil)

	ready, err := DeploymentIsReady(factory, "velero")
	require.NoError(t, err)
	assert.True(t, ready)
}

func TestDaemonSetIsReady(t *testing.T) {
	daemonset := &appsv1.DaemonSet{
		Status: appsv1.DaemonSetStatus{
			NumberAvailable:        1,
			DesiredNumberScheduled: 1,
		},
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(daemonset)
	require.NoError(t, err)

	dc := &test.FakeDynamicClient{}
	dc.On("Get", mock.Anything, mock.Anything).Return(&unstructured.Unstructured{Object: obj}, nil)

	factory := &test.FakeDynamicFactory{}
	factory.On("ClientForGroupVersionResource", mock.Anything, mock.Anything, mock.Anything).Return(dc, nil)

	ready, err := DaemonSetIsReady(factory, "velero")
	require.NoError(t, err)
	assert.True(t, ready)
}

// Prevent https://github.com/vmware-tanzu/velero/issues/8157
func TestPkgImportNoCloudProvider(t *testing.T) {
	_, filename, _, ok := goruntime.Caller(0)
	if !ok {
		t.Fatalf("No caller information")
	}
	t.Logf("Current test file path: %s", filename)
	t.Logf("Current test directory: %s", filepath.Dir(filename)) // should be "pkg/install"
	// go list -f {{.Deps}} ./pkg/install/
	cmd := exec.Command(
		"go",
		"list",
		"-f",
		"{{.Deps}}",
		".",
	)
	// set cmd.Dir to this package even if executed from different dir
	cmd.Dir = filepath.Dir(filename)
	output, err := cmd.Output()
	require.NoError(t, err)
	// split dep by line, replace space with newline
	deps := strings.ReplaceAll(string(output), " ", "\n")
	require.NotEmpty(t, deps)
	// ignore k8s.io
	k8sio, err := regexp.Compile("^k8s.io")
	require.NoError(t, err)
	cloudProvider, err := regexp.Compile("aws|gcp|azure")
	require.NoError(t, err)
	// depsArr :=[]string{}
	cloudProviderDeps := []string{}
	for _, dep := range strings.Split(deps, "\n") {
		if !k8sio.MatchString(dep) {
			// depsArr = append(depsArr, dep)
			if cloudProvider.MatchString(dep) {
				cloudProviderDeps = append(cloudProviderDeps, dep)
			}
		}
	}
	// TODO: expected to fail until #8145 rebase
	require.Empty(t, cloudProviderDeps)
}
