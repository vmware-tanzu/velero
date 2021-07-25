package e2e

import (
	"context"
	"flag"
	"fmt"
	"time"
	"strings"

	"github.com/google/uuid"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("[Basic] Backup/restore of Namespaced Scoped and Cluster Scoped RBAC", func() {

	client, err := newTestClient()
	Expect(err).To(Succeed(), "Failed to instantiate cluster client for rbac test")

	BeforeEach(func() {
		var err error
		flag.Parse()
		uuidgen, err = uuid.NewRandom()
		Expect(err).To(Succeed())
		if installVelero {
			Expect(veleroInstall(context.Background(), veleroImage, veleroNamespace, cloudProvider, objectStoreProvider, false,
				cloudCredentialsFile, bslBucket, bslPrefix, bslConfig, vslConfig, "")).To(Succeed())

		}
	})

	AfterEach(func() {
		if installVelero {
			timeoutCTX, _ := context.WithTimeout(context.Background(), time.Minute)
			err := veleroUninstall(timeoutCTX, client.kubebuilder, installVelero, veleroNamespace)
			Expect(err).To(Succeed())
		}

	})

	Context("When I create Namespaced Scoped and Cluster Scoped RBAC", func() {
		It("should be successfully backed up and restored", func() {
			backupName := "backup-" + uuidgen.String()
			restoreName := "restore-" + uuidgen.String()
			tenMinTimeout, _ := context.WithTimeout(context.Background(), 10*time.Minute)
			Expect(RunRBACTest(tenMinTimeout, client, "test-"+uuidgen.String(), 5,
				backupName, restoreName)).To(Succeed(), "Failed to successfully backup and restore RBAC" )
		})
	})
})

func RunRBACTest(ctx context.Context, client testClient, nsBaseName string, numberOfNamespaces int, backupName string, restoreName string) error {

	//defer does not run until the actual surrounding function returns a value, this line is meant for post test cleanup
	defer cleanupNamespaces(ctx, client, nsBaseName) // Run at exit for final cleanup
	var excludeNamespaces []string

	// Currently it's hard to build a large list of namespaces to include and wildcards do not work so instead
	// we will exclude all of the namespaces that existed prior to the test from the backup
	namespaces, err := client.clientGo.CoreV1().Namespaces().List(ctx, v1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "Could not retrieve namespaces")
	}

	//appending current namespaces to list of excluded namespaces, and no operations will be performed on those
	for _, excludeNamespace := range namespaces.Items {
		excludeNamespaces = append(excludeNamespaces, excludeNamespace.Name)
	}

	//creates the test namespaces mentioned in arguments of the function, with format of test-someuuid. This loops runs up to the number mentioned in the argument
	for nsNum := 0; nsNum < numberOfNamespaces; nsNum++ {
		createNSName := fmt.Sprintf("%s-%00000d", nsBaseName, nsNum)
		createServiceAccountName := fmt.Sprintf("service-account-%s-%00000d", nsBaseName, nsNum)
		createClusterRoleName := fmt.Sprintf("clusterrole-%s-%00000d", nsBaseName, nsNum)
		createClusterRoleBindingName := fmt.Sprintf("clusterrolebinding-%s-%00000d", nsBaseName, nsNum)
		if err := createNamespaceAndRbac(ctx, client, createNSName, createServiceAccountName,createClusterRoleName, createClusterRoleBindingName); err != nil {
			return errors.Wrapf(err, "Failed to create test resources")
		}
	}

	if err := veleroBackupExcludeNamespaces(ctx, veleroCLI, veleroNamespace, backupName, excludeNamespaces); err != nil {
		veleroBackupLogs(ctx, veleroCLI, "", backupName)
		return errors.Wrapf(err, "Failed to backup backup namespaces %s-*", nsBaseName)
	}

	//if error in cleaning up namespaces, then throw error
	err = cleanupNamespaces(ctx, client, nsBaseName)
	if err != nil {
		return errors.Wrap(err, "Could not cleanup namespaces")
	}
	//cleanup clusterrole
	err = cleanupClusterRole(ctx, client, nsBaseName)
	if err != nil {
		return errors.Wrap(err, "Could not cleanup clusterroles")
	}

	//cleanup cluster rolebinding
	err = cleanupClusterRoleBinding(ctx, client, nsBaseName)
	if err != nil {
		return errors.Wrap(err, "Could not cleanup clusterrolebindings")
	}


	//if error in restoring namespaces, then throw error
	err = veleroRestore(ctx, veleroCLI, veleroNamespace, restoreName, backupName)
	if err != nil {
		return errors.Wrap(err, "Restore failed")
	}

	// Verify that we got back all of the namespaces, service accounts, clusterroles and clusterrolebindings we created. Also check if the clusterrolebinding mappings are correct.
	for nsNum := 0; nsNum < numberOfNamespaces; nsNum++ {
		checkNSName := fmt.Sprintf("%s-%00000d", nsBaseName, nsNum)
		checkServiceAccountName := fmt.Sprintf("service-account-%s-%00000d", nsBaseName, nsNum)
		checkClusterRoleName := fmt.Sprintf("clusterrole-%s-%00000d", nsBaseName, nsNum)
		checkClusterRoleBindingName := fmt.Sprintf("clusterrolebinding-%s-%00000d", nsBaseName, nsNum)
		checkNS, err := getNamespace(ctx, client, checkNSName)


		if err != nil {
			return errors.Wrapf(err, "Could not retrieve test namespace %s", checkNSName)
		}

		if checkNS.Name != checkNSName {
			return errors.Errorf("Retrieved namespace for %s has name %s instead", checkNSName, checkNS.Name)
		}

		
		//getting service account from the restore
		checkSA, err := getServiceAccount(ctx, client, checkNSName ,checkServiceAccountName)

		if err != nil {
			return errors.Wrapf(err, "Could not retrieve test service account %s", checkSA)
		}

		if checkSA.Name != checkServiceAccountName {
			return errors.Errorf("Retrieved service account for %s has name %s instead", checkServiceAccountName, checkSA.Name)
		}


		//getting cluster role from the restore
		checkClusterRole, err := getClusterRole(ctx, client,checkClusterRoleName)

		if err != nil {
			return errors.Wrapf(err, "Could not retrieve test cluster role %s", checkClusterRole)
		}

		if checkSA.Name != checkServiceAccountName {
			return errors.Errorf("Retrieved cluster role for %s has name %s instead", checkClusterRoleName, checkClusterRole.Name)
		}


		//getting cluster role binding from the restore
		checkClusterRoleBinding, err := getClusterRoleBinding(ctx, client,checkClusterRoleBindingName)

		if err != nil {
			return errors.Wrapf(err, "Could not retrieve test cluster role binding %s", checkClusterRoleBinding)
		}

		if checkClusterRoleBinding.Name != checkClusterRoleBindingName {
			return errors.Errorf("Retrieved cluster role binding for %s has name %s instead", checkClusterRoleBindingName, checkClusterRoleBinding.Name)
		}


		//check if the role binding maps to service account
		checkSubjects := checkClusterRoleBinding.Subjects[0].Name

		if checkSubjects != checkServiceAccountName {
			return errors.Errorf("Retrieved cluster role binding for %s has name %s instead", checkServiceAccountName, checkSubjects)
		}


	}
	// Cleanup is automatic on the way out
	return nil
}

func cleanupClusterRole(ctx context.Context, client testClient, nsBaseName string) error {

	clusterroles, err := client.clientGo.RbacV1().ClusterRoles().List(ctx, v1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "Could not retrieve clusterroles")
	}

	for _, checkClusterRole := range clusterroles.Items {
		if strings.HasPrefix(checkClusterRole.Name, "clusterrole-"+nsBaseName) {
			fmt.Printf("Cleaning up clusterrole %s\n", checkClusterRole.Name)
			err = client.clientGo.RbacV1().ClusterRoles().Delete(ctx, checkClusterRole.Name, v1.DeleteOptions{})
			if err != nil {
				return errors.Wrapf(err, "Could not delete clusterrole %s", checkClusterRole.Name)
			}
		}
	}
	return nil
}

func cleanupClusterRoleBinding(ctx context.Context, client testClient, nsBaseName string) error {

	clusterrolebindings, err := client.clientGo.RbacV1().ClusterRoleBindings().List(ctx, v1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "Could not retrieve clusterrolebindings")
	}

	for _, checkClusterRoleBinding := range clusterrolebindings.Items {
		if strings.HasPrefix(checkClusterRoleBinding.Name, "clusterrolebinding-"+nsBaseName) {
			fmt.Printf("Cleaning up clusterrolebinding %s\n", checkClusterRoleBinding.Name)
			err = client.clientGo.RbacV1().ClusterRoleBindings().Delete(ctx, checkClusterRoleBinding.Name, v1.DeleteOptions{})
			if err != nil {
				return errors.Wrapf(err, "Could not delete clusterrolebinding %s", checkClusterRoleBinding.Name)
			}
		}
	}
	return nil
}