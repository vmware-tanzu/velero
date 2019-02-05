/*
Copyright 2018 the Heptio Ark contributors.

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

package restore

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/util/kube"
)

type serviceAccountAction struct {
	logger logrus.FieldLogger
}

func NewServiceAccountAction(logger logrus.FieldLogger) ItemAction {
	return &serviceAccountAction{logger: logger}
}

func (a *serviceAccountAction) AppliesTo() (ResourceSelector, error) {
	return ResourceSelector{
		IncludedResources: []string{"serviceaccounts"},
	}, nil
}

func (a *serviceAccountAction) Execute(obj runtime.Unstructured, restore *api.Restore) (runtime.Unstructured, error, error) {
	a.logger.Info("Executing serviceAccountAction")
	defer a.logger.Info("Done executing serviceAccountAction")

	var serviceAccount corev1.ServiceAccount
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &serviceAccount); err != nil {
		return nil, nil, errors.Wrap(err, "unable to convert serviceaccount from runtime.Unstructured")
	}

	log := a.logger.WithField("serviceaccount", kube.NamespaceAndName(&serviceAccount))

	log.Debug("Checking secrets")
	check := serviceAccount.Name + "-token-"
	for i := len(serviceAccount.Secrets) - 1; i >= 0; i-- {
		secret := &serviceAccount.Secrets[i]
		log.Debugf("Checking if secret %s matches %s", secret.Name, check)

		if strings.HasPrefix(secret.Name, check) {
			// Copy all secrets *except* -token-
			log.Debug("Match found - excluding this secret")
			serviceAccount.Secrets = append(serviceAccount.Secrets[:i], serviceAccount.Secrets[i+1:]...)
			break
		} else {
			log.Debug("No match found - including this secret")
		}
	}

	res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&serviceAccount)
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to convert serviceaccount to runtime.Unstructured")
	}

	return &unstructured.Unstructured{Object: res}, nil, nil
}
