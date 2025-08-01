// Code generated by mockery v2.35.4. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"
	labels "k8s.io/apimachinery/pkg/labels"

	v1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/listers/volumesnapshot/v1"
)

// VolumeSnapshotLister is an autogenerated mock type for the VolumeSnapshotLister type
type VolumeSnapshotLister struct {
	mock.Mock
}

// List provides a mock function with given fields: selector
func (_m *VolumeSnapshotLister) List(selector labels.Selector) ([]*v1.VolumeSnapshot, error) {
	ret := _m.Called(selector)

	var r0 []*v1.VolumeSnapshot
	var r1 error
	if rf, ok := ret.Get(0).(func(labels.Selector) ([]*v1.VolumeSnapshot, error)); ok {
		return rf(selector)
	}
	if rf, ok := ret.Get(0).(func(labels.Selector) []*v1.VolumeSnapshot); ok {
		r0 = rf(selector)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*v1.VolumeSnapshot)
		}
	}

	if rf, ok := ret.Get(1).(func(labels.Selector) error); ok {
		r1 = rf(selector)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// VolumeSnapshots provides a mock function with given fields: namespace
func (_m *VolumeSnapshotLister) VolumeSnapshots(namespace string) volumesnapshotv1.VolumeSnapshotNamespaceLister {
	ret := _m.Called(namespace)

	var r0 volumesnapshotv1.VolumeSnapshotNamespaceLister
	if rf, ok := ret.Get(0).(func(string) volumesnapshotv1.VolumeSnapshotNamespaceLister); ok {
		r0 = rf(namespace)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(volumesnapshotv1.VolumeSnapshotNamespaceLister)
		}
	}

	return r0
}

// NewVolumeSnapshotLister creates a new instance of VolumeSnapshotLister. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewVolumeSnapshotLister(t interface {
	mock.TestingT
	Cleanup(func())
}) *VolumeSnapshotLister {
	mock := &VolumeSnapshotLister{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
