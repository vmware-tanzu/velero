package resourcepolicies

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kbClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// resPoliciesMeta stored one resource policies and related resource policies configmap creation timestamp
type resPoliciesMeta struct {
	policies              *ResourcePolicies
	configResourceVersion string
}

// resPoliciesKey identify one specific resPoliciesMeta with refName and refType
type resPoliciesKey struct {
	refName string
	refType string
}

// ResPoliciesManager manage all resource policies in memory and keep all up-to-date
type ResPoliciesManager struct {
	resPolicies map[resPoliciesKey]*resPoliciesMeta
	namespace   string
	kbClient    kbClient.Client
}

var ResPoliciesMgr ResPoliciesManager

func (mgr *ResPoliciesManager) InitResPoliciesMgr(namespace string, kbClient kbClient.Client) {
	mgr.resPolicies = make(map[resPoliciesKey]*resPoliciesMeta)
	mgr.namespace = namespace
	mgr.kbClient = kbClient
}

// getResourcePolicies first check and update current resource policies and then get the latest resource policies
func (mgr *ResPoliciesManager) getResourcePolicies(refName, refType string) (*ResourcePolicies, error) {
	// TODO currently we only want to support type of configmap
	if refType != "configmap" {
		return nil, fmt.Errorf("unsupported resource policies reference type %s", refType)
	}

	resKey := resPoliciesKey{
		refName: refName,
		refType: refType,
	}
	policiesConfigmap := &v1.ConfigMap{}
	err := mgr.kbClient.Get(context.Background(), kbClient.ObjectKey{Namespace: mgr.namespace, Name: refName}, policiesConfigmap)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get resource policies %s/%s configmap", refName, mgr.namespace)
	}
	if policiesConfigmap == nil {
		return nil, errors.Wrapf(err, "get empty resource policies %s/%s configmap", refName, mgr.namespace)
	}

	updateResPolicies := func() (*ResourcePolicies, error) {
		res, err := GetResourcePoliciesFromConfig(policiesConfigmap)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get the user resource policies config")
		}
		if res == nil {
			return nil, fmt.Errorf("%v resource policy not found", resKey)
		}
		// if exist it will overwrite or it will add
		mgr.resPolicies[resKey] = &resPoliciesMeta{
			policies:              res,
			configResourceVersion: policiesConfigmap.GetResourceVersion(),
		}
		return res, nil
	}

	if policies, exist := mgr.resPolicies[resKey]; exist {
		// check if it needs update
		if policiesConfigmap.ResourceVersion > policies.configResourceVersion {
			res, err := updateResPolicies()
			if err != nil {
				return nil, err
			}
			return res, nil
		}
		// exist and already the latest
		return policies.policies, nil
	} else {
		// not exist
		res, err := updateResPolicies()
		if err != nil {
			return nil, err
		}
		return res, nil
	}
}

func (mgr *ResPoliciesManager) GetStructredVolumeFromPVC(item runtime.Unstructured) (*StructuredVolume, error) {
	pvc := v1.PersistentVolumeClaim{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &pvc); err != nil {
		return nil, errors.WithStack(err)
	}

	pvName := pvc.Spec.VolumeName
	if pvName == "" {
		return nil, errors.Errorf("PVC has no volume backing this claim")
	}

	pv := &v1.PersistentVolume{}
	if err := mgr.kbClient.Get(context.Background(), kbClient.ObjectKey{Name: pvName}, pv); err != nil {
		return nil, errors.WithStack(err)
	}
	volume := StructuredVolume{}
	volume.ParsePV(pv)
	return &volume, nil
}

func (mgr *ResPoliciesManager) GetVolumeMatchedAction(refName, refType string, volume *StructuredVolume) (*Action, error) {
	resPolicies, err := mgr.getResourcePolicies(refName, refType)
	if err != nil {
		return nil, err
	}
	if resPolicies == nil {
		return nil, fmt.Errorf("failed to get resource policies %s/%s configmap", refName, mgr.namespace)
	}

	return getVolumeMatchedAction(resPolicies, volume), nil
}
