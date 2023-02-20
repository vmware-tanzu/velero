package basic

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"

	. "github.com/vmware-tanzu/velero/test/e2e"
	. "github.com/vmware-tanzu/velero/test/e2e/test"
	. "github.com/vmware-tanzu/velero/test/e2e/util/k8s"
	. "github.com/vmware-tanzu/velero/test/e2e/util/velero"
)

type NodePort struct {
	TestCase
	replica              int32
	labels               map[string]string
	containers           *[]v1.Container
	serviceName          string
	serviceSpec          *v1.ServiceSpec
	nodePort             int32
	namespaceToCollision string
	namespace            string
}

const NodeportBaseName string = "nodeport-"

var NodePortTest func() = TestFunc(&NodePort{namespace: NodeportBaseName + "1", TestCase: TestCase{NSBaseName: NodeportBaseName}})

func (n *NodePort) Init() error {
	n.VeleroCfg = VeleroCfg
	n.Client = *n.VeleroCfg.ClientToInstallVelero
	n.NSBaseName = NodeportBaseName
	n.NamespacesTotal = 1
	n.TestMsg = &TestMSG{
		Desc:      fmt.Sprintf("Nodeport preservation"),
		FailedMSG: "Failed to restore with nodeport preservation",
		Text:      fmt.Sprintf("Nodeport can be preserved or omit during restore"),
	}
	n.BackupName = "backup-label-selector-" + UUIDgen.String()
	n.RestoreName = "restore-" + UUIDgen.String()
	n.serviceName = "nginx-service-" + UUIDgen.String()
	n.labels = map[string]string{"app": "nginx"}
	return nil
}

func (n *NodePort) StartRun() error {
	n.BackupArgs = []string{
		"create", "--namespace", VeleroCfg.VeleroNamespace, "backup", n.BackupName,
		"--include-namespaces", n.namespace, "--wait",
	}
	n.RestoreArgs = []string{
		"create", "--namespace", VeleroCfg.VeleroNamespace, "restore",
		"--from-backup", n.BackupName,
		"--wait",
	}
	return nil
}
func (n *NodePort) CreateResources() error {
	n.Ctx, _ = context.WithTimeout(context.Background(), 60*time.Minute)

	By(fmt.Sprintf("Creating service %s in namespaces %s ......\n", n.serviceName, n.namespace), func() {
		Expect(CreateNamespace(n.Ctx, n.Client, n.namespace)).To(Succeed(), fmt.Sprintf("Failed to create namespace %s", n.namespace))
		Expect(createServiceWithNodeport(n.Ctx, n.Client, n.namespace, n.serviceName, n.labels, 0)).To(Succeed(), fmt.Sprintf("Failed to create service %s", n.serviceName))
		service, err := GetService(n.Ctx, n.Client, n.namespace, n.serviceName)
		Expect(err).To(Succeed())
		Expect(len(service.Spec.Ports)).To(Equal(1))
		n.nodePort = service.Spec.Ports[0].NodePort
		_, err = GetAllService(n.Ctx)
		Expect(err).To(Succeed(), "fail to get service")
	})
	return nil
}

func (n *NodePort) Destroy() error {
	By(fmt.Sprintf("Start to destroy namespace %s......", n.NSBaseName), func() {
		Expect(CleanupNamespacesWithPoll(n.Ctx, n.Client, NodeportBaseName)).To(Succeed(),
			fmt.Sprintf("Failed to delete namespace %s", n.NSBaseName))
		Expect(WaitForServiceDelete(n.Client, n.namespace, n.serviceName, false)).To(Succeed(), "fail to delete service")
		_, err := GetAllService(n.Ctx)
		Expect(err).To(Succeed(), "fail to get service")
	})

	n.namespaceToCollision = NodeportBaseName + "tmp"
	By(fmt.Sprintf("Creating a new service which has the same nodeport as backed up service has in a new namespaces for nodeport collision ...%s\n", n.namespaceToCollision), func() {
		Expect(CreateNamespace(n.Ctx, n.Client, n.namespaceToCollision)).To(Succeed(), fmt.Sprintf("Failed to create namespace %s", n.namespaceToCollision))
		Expect(createServiceWithNodeport(n.Ctx, n.Client, n.namespaceToCollision, n.serviceName, n.labels, n.nodePort)).To(Succeed(), fmt.Sprintf("Failed to create service %s", n.serviceName))
		_, err := GetAllService(n.Ctx)
		Expect(err).To(Succeed(), "fail to get service")
	})

	return nil
}

func (n *NodePort) Restore() error {
	index := 4
	restoreName1 := n.RestoreName + "-1"
	restoreName2 := restoreName1 + "-1"

	args := n.RestoreArgs
	args = append(args[:index], append([]string{n.RestoreName}, args[index:]...)...)
	args = append(args, "--preserve-nodeports=true")
	By(fmt.Sprintf("Start to restore %s with nodeports preservation when port %d is already occupied by other service", n.RestoreName, n.nodePort), func() {
		Expect(VeleroRestoreExec(n.Ctx, n.VeleroCfg.VeleroCLI,
			n.VeleroCfg.VeleroNamespace, n.RestoreName,
			args, velerov1api.RestorePhasePartiallyFailed)).To(
			Succeed(),
			func() string {
				RunDebug(context.Background(), n.VeleroCfg.VeleroCLI,
					n.VeleroCfg.VeleroNamespace, "", n.RestoreName)
				return "Fail to restore workload"
			})
	})

	args = n.RestoreArgs
	args = append(args[:index], append([]string{restoreName1}, args[index:]...)...)
	args = append(args, "--preserve-nodeports=false")
	By(fmt.Sprintf("Start to restore %s without nodeports preservation ......", restoreName1), func() {
		Expect(VeleroRestoreExec(n.Ctx, n.VeleroCfg.VeleroCLI, n.VeleroCfg.VeleroNamespace,
			restoreName1, args, velerov1api.RestorePhaseCompleted)).To(Succeed(), func() string {
			RunDebug(context.Background(), n.VeleroCfg.VeleroCLI, n.VeleroCfg.VeleroNamespace, "", restoreName1)
			return "Fail to restore workload"
		})
	})

	By(fmt.Sprintf("Delete service %s by deleting namespace %s", n.serviceName, n.namespace), func() {
		service, err := GetService(n.Ctx, n.Client, n.namespace, n.serviceName)
		Expect(err).To(Succeed())
		Expect(len(service.Spec.Ports)).To(Equal(1))
		fmt.Println(service.Spec.Ports)
		Expect(DeleteNamespace(n.Ctx, n.Client, n.namespace, true)).To(Succeed())
	})

	By(fmt.Sprintf("Start to delete service %s in namespace %s ......", n.serviceName, n.namespaceToCollision), func() {
		Expect(WaitForServiceDelete(n.Client, n.namespaceToCollision, n.serviceName, true)).To(Succeed(), "fail to delete service")
		_, err := GetAllService(n.Ctx)
		Expect(err).To(Succeed(), "fail to get service")
	})

	args = n.RestoreArgs
	args = append(args[:index], append([]string{restoreName2}, args[index:]...)...)
	args = append(args, "--preserve-nodeports=true")
	By(fmt.Sprintf("Start to restore %s with nodeports preservation ......", restoreName2), func() {
		Expect(VeleroRestoreExec(n.Ctx, n.VeleroCfg.VeleroCLI, n.VeleroCfg.VeleroNamespace,
			restoreName2, args, velerov1api.RestorePhaseCompleted)).To(Succeed(), func() string {
			RunDebug(context.Background(), n.VeleroCfg.VeleroCLI, n.VeleroCfg.VeleroNamespace, "", restoreName2)
			return "Fail to restore workload"
		})
	})

	By(fmt.Sprintf("Verify service %s was restore successfully with the origin nodeport.", n.namespace), func() {
		service, err := GetService(n.Ctx, n.Client, n.namespace, n.serviceName)
		Expect(err).To(Succeed())
		Expect(len(service.Spec.Ports)).To(Equal(1))
		Expect(service.Spec.Ports[0].NodePort).To(Equal(n.nodePort))
	})

	return nil
}

func createServiceWithNodeport(ctx context.Context, client TestClient, namespace string,
	service string, labels map[string]string, nodePort int32) error {
	serviceSpec := &v1.ServiceSpec{
		Ports: []v1.ServicePort{
			{
				Port:       80,
				TargetPort: intstr.IntOrString{IntVal: 80},
				NodePort:   nodePort,
			},
		},
		Selector: nil,
		Type:     v1.ServiceTypeLoadBalancer,
	}
	return CreateService(ctx, client, namespace, service, labels, serviceSpec)
}
