package e2etestify

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"
	"github.com/vmware-tanzu/velero/test/e2e"
)

func (suite *RestorePriorityAPIGVTests) TestCases() {
	tests := []struct {
		name       string
		namespaces []string
		srcCRD     map[string]string
		srcCRs     map[string]string
		tgtCRD     map[string]string
		tgtVer     string
		cm         *corev1api.ConfigMap
		gvs        map[string][]string
		want       map[string]map[string]string
	}{
		{
			name: "Target and source cluster preferred versions match; Preferred version v1 is restored (Priority 1, Case A).",
			srcCRD: map[string]string{
				"url":       "https://raw.githubusercontent.com/brito-rafa/k8s-webhooks/master/examples-for-projectvelero/case-a/source/case-a-source.yaml",
				"namespace": "music-system",
			},
			srcCRs: map[string]string{
				"v1":       "https://raw.githubusercontent.com/brito-rafa/k8s-webhooks/master/examples-for-projectvelero/case-a/source/music/config/samples/music_v1_rockband.yaml",
				"v1alpha1": "https://raw.githubusercontent.com/brito-rafa/k8s-webhooks/master/examples-for-projectvelero/case-a/source/music/config/samples/music_v1alpha1_rockband.yaml",
			},
			tgtCRD: map[string]string{
				"url":       "https://raw.githubusercontent.com/brito-rafa/k8s-webhooks/master/examples-for-projectvelero/case-a/target/case-a-target.yaml",
				"namespace": "music-system",
			},
			tgtVer: "v1",
			cm:     nil,
			want: map[string]map[string]string{
				"annotations": {
					"rockbands.music.example.io/originalVersion": "v1",
				},
				"specs": {
					"leadSinger": "John Lennon",
				},
			},
		},
		{
			name: "Latest common non-preferred supported version v2beta2 is restored (Priority 3, Case D).",
			srcCRD: map[string]string{
				"url":       "https://raw.githubusercontent.com/brito-rafa/k8s-webhooks/master/examples-for-projectvelero/case-b/source/case-b-source-manually-added-mutations.yaml",
				"namespace": "music-system",
			},
			srcCRs: map[string]string{
				"v2beta2": "https://raw.githubusercontent.com/brito-rafa/k8s-webhooks/master/examples-for-projectvelero/case-b/source/music/config/samples/music_v2beta2_rockband.yaml",
				"v2beta1": "https://raw.githubusercontent.com/brito-rafa/k8s-webhooks/master/examples-for-projectvelero/case-b/source/music/config/samples/music_v2beta1_rockband.yaml",
				"v1":      "https://raw.githubusercontent.com/brito-rafa/k8s-webhooks/master/examples-for-projectvelero/case-b/source/music/config/samples/music_v1_rockband.yaml",
			},
			tgtCRD: map[string]string{
				"url":       "https://raw.githubusercontent.com/brito-rafa/k8s-webhooks/master/examples-for-projectvelero/case-d/target/case-d-target-manually-added-mutations.yaml",
				"namespace": "music-system",
			},
			tgtVer: "v2beta2",
			cm:     nil,
			want: map[string]map[string]string{
				"annotations": {
					"rockbands.music.example.io/originalVersion": "v2beta2",
				},
				"specs": {
					"leadSinger": "John Lennon",
					"leadGuitar": "George Harrison",
					"drummer":    "Ringo Starr",
				},
			},
		},
		{
			name: "No common supported versions means no rockbands custom resource is restored.",
			srcCRD: map[string]string{
				"url":       "https://raw.githubusercontent.com/brito-rafa/k8s-webhooks/master/examples-for-projectvelero/case-a/source/case-a-source.yaml",
				"namespace": "music-system",
			},
			srcCRs: map[string]string{
				"v1":       "https://raw.githubusercontent.com/brito-rafa/k8s-webhooks/master/examples-for-projectvelero/case-a/source/music/config/samples/music_v1_rockband.yaml",
				"v1alpha1": "https://raw.githubusercontent.com/brito-rafa/k8s-webhooks/master/examples-for-projectvelero/case-a/source/music/config/samples/music_v1alpha1_rockband.yaml",
			},
			tgtCRD: map[string]string{
				"url":       "https://raw.githubusercontent.com/brito-rafa/k8s-webhooks/master/examples-for-projectvelero/case-b/target/case-b-target-manually-added-mutations.yaml",
				"namespace": "music-system",
			},
			tgtVer: "",
			cm:     nil,
			want:   nil,
		},
	}

	ctx := context.Background()

	for i, tc := range tests {
		fmt.Printf("\n====== Test Case %d ======\n", i)

		err := InstallCRD(ctx, tc.srcCRD["url"], tc.srcCRD["namespace"])
		if err != nil {
			suite.Require().NoError(err, "installing music-system CRD for source cluster")
		}

		for v, cr := range tc.srcCRs {
			ns := suite.resource + "-src-" + v

			if err := e2e.CreateNamespace(ctx, suite.client, ns); err != nil {
				suite.Require().NoErrorf(err, "creating %s namespace", ns)
			}

			if err := InstallCR(ctx, cr, ns); err != nil {
				suite.Require().NoErrorf(err, "installing %s custom resource on source cluster namespace %s", cr, ns)
			}

			tc.namespaces = append(tc.namespaces, ns)
		}

		suite.Require().NoError(installVelero(ctx), "install velero")

		// Reset Velero to recognize music-system CRD.
		cmd := exec.CommandContext(ctx, "kubectl", "delete", "pod", "--all", "-n", "velero")
		_, _, _ = veleroexec.RunCommand(cmd)

		fmt.Println("Sleep 40s to wait for Velero install to stabilize.")
		time.Sleep(time.Second * 40)

		backup := "backup-rockbands-" + suite.uuidgen.String() + "-" + strconv.Itoa(i)
		namespacesStr := strings.Join(tc.namespaces, ",")

		err = e2e.VeleroBackupNamespace(ctx, *veleroCLI, backup, namespacesStr)
		if err != nil {
			suite.Require().NoErrorf(err, "backing up %s namespaces on source cluster", namespacesStr)
		}

		// Delete music-system CRD and controllers installed on source cluster.
		if err := DeleteCRD(ctx, tc.srcCRD["url"], tc.srcCRD["namespace"]); err != nil {
			suite.Require().NoError(err, "deleting music-system CRD from source cluster")
		}

		for _, ns := range tc.namespaces {
			if err := suite.client.CoreV1().Namespaces().Delete(ctx, ns, metav1.DeleteOptions{}); err != nil {
				suite.Require().NoError(err, "deleting %s namespace from source cluster", ns)
			}

			if err := WaitNamespaceDelete(ctx, ns); err != nil {
				suite.Require().NoError(err, "deleting %s namespace from source cluster", ns)
			}
		}

		// Install music-system CRD for target cluster.
		if err := InstallCRD(ctx, tc.tgtCRD["url"], tc.tgtCRD["namespace"]); err != nil {
			suite.Require().NoError(err, "installing music-system CRD for target cluster")
		}

		// Reset Velero to recognize music-system CRD.
		cmd = exec.CommandContext(ctx, "kubectl", "delete", "pod", "--all", "-n", "velero")
		_, _, _ = veleroexec.RunCommand(cmd)

		fmt.Println("Sleep 20s to wait for Velero install to stabilize.")
		time.Sleep(time.Second * 20)

		// Restore rockbands namespace.
		restore := "restore-rockbands-" + suite.uuidgen.String() + "-" + strconv.Itoa(i)

		if tc.want != nil {
			if err := e2e.VeleroRestore(ctx, *veleroCLI, restore, backup); err != nil {
				suite.Require().NoErrorf(err, "restoring %s namespaces on target cluster", namespacesStr)
			}

			annoSpec, err := resourceInfo(ctx, suite.group, tc.tgtVer, suite.resource)
			if err != nil {
				suite.Require().NoErrorf(err, "get annotation and spec from %s.%s/%s object", suite.resource, suite.group, tc.tgtVer)
			}

			suite.Assert().Equal(
				true,
				containsAll(annoSpec["annotations"], tc.want["annotations"]),
				"actual annotations:",
				annoSpec["annotations"],
				"expected annotations:",
				tc.want["annotations"],
			)
			suite.Assert().Equal(
				true,
				containsAll(annoSpec["specs"], tc.want["specs"]),
				"actual specs:",
				annoSpec["specs"],
				"expected specs:",
				tc.want["specs"],
			)
		} else {
			// No custom resource should have been restored. Expect "no resource found"
			// error during restore.
			err := e2e.VeleroRestore(ctx, *veleroCLI, restore, backup)
			// TODO: fill expected error
			suite.Assert().EqualError(err, "Unexpected restore phase got PartiallyFailed, expecting Completed")
		}

		// Delete namespaces created for CRs
		for _, ns := range tc.namespaces {
			fmt.Println("Delete namespace", ns)
			_ = suite.client.CoreV1().Namespaces().Delete(ctx, ns, metav1.DeleteOptions{})
			_ = WaitNamespaceDelete(ctx, ns)
		}

		// Delete source cluster music-system CRD
		_ = DeleteCRD(
			ctx,
			tc.srcCRD["url"],
			tc.srcCRD["namespace"],
		)

		// Delete target cluster music-system CRD
		_ = DeleteCRD(
			ctx,
			tc.tgtCRD["url"],
			tc.srcCRD["namespace"],
		)
	}
}
