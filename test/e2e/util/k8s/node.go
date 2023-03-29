package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	common "github.com/vmware-tanzu/velero/test/e2e/util/common"
)

func GetWorkerNodes(ctx context.Context) ([]string, error) {
	getCMD := exec.CommandContext(ctx, "kubectl", "get", "node", "-o", "json")

	fmt.Printf("kubectl get node cmd =%v\n", getCMD)
	jsonBuf, err := common.CMDExecWithOutput(getCMD)
	if err != nil {
		return nil, err
	}

	nodes := &unstructured.UnstructuredList{}
	err = json.Unmarshal(*jsonBuf, &nodes)
	if err != nil {
		return nil, err
	}
	var nodeNameList []string
	for nodeIndex, node := range nodes.Items {
		// := v1.Node{}
		fmt.Println(nodeIndex)
		fmt.Println(node.GetName())
		anns := node.GetAnnotations()
		fmt.Println(anns)
		fmt.Println(anns["cluster.x-k8s.io/owner-kind"])
		//"MachineSet"
		if anns["cluster.x-k8s.io/owner-kind"] == "KubeadmControlPlane" {
			continue
		}
		nodeNameList = append(nodeNameList, node.GetName())
	}
	return nodeNameList, nil
}
