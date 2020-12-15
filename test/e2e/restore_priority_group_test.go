package e2e

import (
	"os/exec"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"
)

func RunRestoreWithAPIGroupVersionsFeatureFlag() error {
	tests := []struct {
		name   string
		srcCRD string
		srcCRs []string
		tgtCRD string
		cm     *corev1api.ConfigMap
		want   map[string][]string
	}{
		{
			name:   "Target and source cluster preferred versions match; Preferred version is restored (Priority 1, Case A).",
			srcCRD: "https://raw.githubusercontent.com/brito-rafa/k8s-webhooks/master/examples-for-projectvelero/case-a/source/case-a-source.yaml",
			srcCRs: []string{
				"https://raw.githubusercontent.com/brito-rafa/k8s-webhooks/master/examples-for-projectvelero/case-a/source/music/config/samples/music_v1_rockband.yaml",
				"https://raw.githubusercontent.com/brito-rafa/k8s-webhooks/master/examples-for-projectvelero/case-a/source/music/config/samples/music_v1alpha1_rockband.yaml",
			},
			tgtCRD: "https://raw.githubusercontent.com/brito-rafa/k8s-webhooks/master/examples-for-projectvelero/case-a/target/case-a-target.yaml",
			cm:     nil,
			want: map[string][]string{
				"annotation": {"rockbands.music.example.io/originalVersion: v1"},
				"leadSinger": {"John Lennon"},
			},
		},
		{
			name: "Target cluster preferred version was supported but not preferred by source, and is restored (Priority 1, Case B).",
		},
		{
			name: "Source cluster preferred version is supported but not preferred by target and is restored (Priority 2, Case C).",
		},
		{
			name: "Latest common non-preferred supported version is restored (Priority 3, Case D).",
		},
		{
			name: "No common supported versions defaults to source cluster preferred version being restored.",
		},
		{
			name: "Choose to backup the API Group version that was priortized by user.",
		},
		{
			name: "Use pre-defined version priorities if user-defined priorities are invalid.",
		},
		{
			name: "Use pre-defined version priorities if there are no user-defined priorities.",
		},
	}

	// Test preparation
	const (
		shortTimeout = 5
		longTimeout  = 20
		namespace    = "rockbands"
	)
	var (
		backupName  string = "backup-rockbands-" + uuidgen.String()
		restoreName string = "restore-rockbands-" + uuidgen.String()
	)

	ctxShort, _ := context.WithTimeout(context.Background(), shortTimeout*time.Minute)
	ctxLong, _ := context.WithTimeout(context.Background(), longTimeout*time.Minute)

	veleroInstall(pluginProvider, false)
	client, err := GetClusterClient()
	if err != nil {
		return errors.Wrap(err, "getting client cluster:")
	}

	installCertManagerCmd := exec.CommandContext(
		ctxShort,
		"kubectl", "apply",
		"-f", "https://github.com/jetstack/cert-manager/releases/download/v1.0.3/cert-manager.yaml",
	)
	_, _, err = veleroexec.RunCommand(installCertManagerCmd)
	if err != nil {
		return errors.Wrap(err, "installing cert-manager")
	}

	for _, tc := range tests {

		if err := CreateNamespace(ctxShort, client, namespace); err != nil {
			return errors.Wrap(err, "creating rockbands namespace")
		}

		// Install music-system CRD on source cluster.
		installSrcCRDCmd := exec.CommandContext(ctxShort, "kubectl", "apply", "-f", tc.srcCRD)
		_, _, err = veleroexec.RunCommand(installSrcCRDCmd)
		if err != nil {
			return errors.Wrap(err, "installing music-system CRD for source cluster")
		}

		for _, cr := range tc.srcCRs {
			installSrcCRCmd := exec.CommandContext(ctxShort, "kubectl", "apply", "-n", namespace, "-f", cr)
			_, _, err = veleroexec.RunCommand(installSrcCRCmd)
			if err != nil {
				return errors.Wrap(err, "installing a custom resource on source cluster")
			}
		}

		if err := VeleroBackupNamespace(ctxLong, veleroCLI, backupName, namespace); err != nil {
			return errors.Wrap(err, "backing up rockbands namespace on source cluster")
		}

		// Delete music-system CRD and controllers installed on source cluster.
		deleteSrcCRDCmd := exec.CommandContext(ctxShort, "kubectl", "delete", "-f", tc.srcCRD)
		_, _, err = veleroexec.RunCommand(deleteSrcCRDCmd)
		if err != nil {
			return errors.Wrap(err, "deleting music-system CRD from source cluster")
		}

		if err := client.CoreV1().Namespaces().Delete(ctxLong, namespace, metav1.DeleteOptions{}); err != nil {
			return errors.Wrap(err, "deleting rockband namespace from source cluster")
		}

		// Install music-system CRD for target cluster.
		installTgtCRDCmd := exec.CommandContext(ctxShort, "kubectl", "apply", "-f", tc.tgtCRD)
		_, _, err = veleroexec.RunCommand(installTgtCRDCmd)
		if err != nil {
			return errors.Wrap(err, "installing music-system CRD for target cluster")
		}

		// Restore rockbands namespace.

		if err := VeleroRestore(ctxLong, veleroCLI, restoreName, backupName); err != nil {
			return errors.Wrap(err, "restoring rockgbands namespace on target cluster")
		}

		// verify annotation
		// get annotations
		// look through annotations
		// ensure matches for every annotation in tc.want["annotation"]

		// verify spec fields key, "leadSinger", and value, tc.want["leadSinger"] match

		// Delete music-system CRD and controllers installed on source cluster.
		deleteTgtCRDCmd := exec.CommandContext(ctxShort, "kubectl", "delete", "-f", tc.tgtCRD)
		_, _, err = veleroexec.RunCommand(deleteTgtCRDCmd)
		if err != nil {
			return errors.Wrap(err, "deleting music-system CRD from target cluster")
		}

		if err := client.CoreV1().Namespaces().Delete(ctxLong, namespace, metav1.DeleteOptions{}); err != nil {
			return errors.Wrap(err, "deleting rockband namespace from target cluster")
		}
	}

	return nil
}
