/*
Copyright the Velero contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

const (
	upgradeNamespace = "upgrade-workload"
)

// runKibishiiTests runs kibishii tests on the provider.
func runUpgradeTests(client testClient, providerName, veleroCLI, veleroNamespace, backupName, restoreName, backupLocation string,
	useVolumeSnapshots bool, registryCredentialFile string, installParams *VeleroInstallationParams) error {
	oneHourTimeout, _ := context.WithTimeout(context.Background(), time.Minute*60)
	serviceAccountName := "default"
	if err := createNamespace(oneHourTimeout, client, upgradeNamespace); err != nil {
		return errors.Wrapf(err, "Failed to create namespace %s to install Kibishii workload", upgradeNamespace)
	}
	defer func() {
		if err := deleteNamespace(oneHourTimeout, client, upgradeNamespace, true); err != nil {
			fmt.Println(errors.Wrapf(err, "failed to delete the namespace %q", upgradeNamespace))
		}
	}()
	// wait until the service account is created before patch the image pull secret
	if err := waitUntilServiceAccountCreated(oneHourTimeout, client, upgradeNamespace, serviceAccountName, 10*time.Minute); err != nil {
		return errors.Wrapf(err, "failed to wait the service account %q created under the namespace %q", serviceAccountName, upgradeNamespace)
	}
	// add the image pull secret to avoid the image pull limit issue of Docker Hub
	if err := patchServiceAccountWithImagePullSecret(oneHourTimeout, client, upgradeNamespace, serviceAccountName, registryCredentialFile); err != nil {
		return errors.Wrapf(err, "failed to patch the service account %q under the namespace %q", serviceAccountName, upgradeNamespace)
	}

	if err := installKibishii(oneHourTimeout, upgradeNamespace, providerName); err != nil {
		return errors.Wrap(err, "Failed to install Kibishii workload")
	}

	// wait for kibishii pod startup
	// TODO - Fix kibishii so we can check that it is ready to go
	fmt.Printf("Waiting for kibishii pods to be ready\n")
	if err := waitForKibishiiPods(oneHourTimeout, client, upgradeNamespace); err != nil {
		return errors.Wrapf(err, "Failed to wait for ready status of kibishii pods in %s", upgradeNamespace)
	}

	if err := generateData(oneHourTimeout, upgradeNamespace, 2, 10, 10, 1024, 1024, 0, 2); err != nil {
		return errors.Wrap(err, "Failed to generate data")
	}

	if err := veleroBackupNamespace(oneHourTimeout, veleroCLI, veleroNamespace, backupName, upgradeNamespace, backupLocation, useVolumeSnapshots); err != nil {
		veleroBackupLogs(oneHourTimeout, veleroCLI, veleroNamespace, backupName)
		return errors.Wrapf(err, "Failed to backup kibishii namespace %s", upgradeNamespace)
	}

	if providerName == "vsphere" && useVolumeSnapshots {
		// Wait for uploads started by the Velero Plug-in for vSphere to complete
		// TODO - remove after upload progress monitoring is implemented
		fmt.Println("Waiting for vSphere uploads to complete")
		if err := waitForVSphereUploadCompletion(oneHourTimeout, time.Hour, upgradeNamespace); err != nil {
			return errors.Wrapf(err, "Error waiting for uploads to complete")
		}
	}
	fmt.Printf("Simulating a disaster by removing namespace %s\n", upgradeNamespace)
	if err := deleteNamespace(oneHourTimeout, client, upgradeNamespace, true); err != nil {
		return errors.Wrapf(err, "failed to delete namespace %s", upgradeNamespace)
	}

	if err := veleroUninstall(context.Background(), client.kubebuilder, true, veleroNamespace); err != nil {
		return errors.Wrapf(err, "Failed to uninstall velero from image")
	}

	if err := veleroInstallNew(installParams); err != nil {
		return errors.Wrapf(err, "Failed to install velero from image %s", installParams.veleroImage)
	}

	if err := veleroRestore(oneHourTimeout, veleroCLI, veleroNamespace, restoreName, backupName); err != nil {
		veleroRestoreLogs(oneHourTimeout, veleroCLI, veleroNamespace, restoreName)
		return errors.Wrapf(err, "Restore %s failed from backup %s", restoreName, backupName)
	}

	// wait for kibishii pod startup
	// TODO - Fix kibishii so we can check that it is ready to go
	fmt.Printf("Waiting for kibishii pods to be ready\n")
	if err := waitForKibishiiPods(oneHourTimeout, client, upgradeNamespace); err != nil {
		return errors.Wrapf(err, "Failed to wait for ready status of kibishii pods in %s", upgradeNamespace)
	}

	// TODO - check that namespace exists
	fmt.Printf("running kibishii verify\n")
	if err := verifyData(oneHourTimeout, upgradeNamespace, 2, 10, 10, 1024, 1024, 0, 2); err != nil {
		return errors.Wrap(err, "Failed to verify data generated by kibishii")
	}

	fmt.Printf("kibishii test completed successfully\n")
	return nil
}
