// Code generated by mockery v2.14.0. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	repository "github.com/vmware-tanzu/velero/pkg/repository"

	time "time"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// Manager is an autogenerated mock type for the Manager type
type Manager struct {
	mock.Mock
}

// ConnectToRepo provides a mock function with given fields: repo
func (_m *Manager) ConnectToRepo(repo *v1.BackupRepository) error {
	ret := _m.Called(repo)

	var r0 error
	if rf, ok := ret.Get(0).(func(*v1.BackupRepository) error); ok {
		r0 = rf(repo)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DefaultMaintenanceFrequency provides a mock function with given fields: repo
func (_m *Manager) DefaultMaintenanceFrequency(repo *v1.BackupRepository) (time.Duration, error) {
	ret := _m.Called(repo)

	var r0 time.Duration
	if rf, ok := ret.Get(0).(func(*v1.BackupRepository) time.Duration); ok {
		r0 = rf(repo)
	} else {
		r0 = ret.Get(0).(time.Duration)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*v1.BackupRepository) error); ok {
		r1 = rf(repo)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Forget provides a mock function with given fields: _a0, _a1
func (_m *Manager) Forget(_a0 context.Context, _a1 repository.SnapshotIdentifier) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, repository.SnapshotIdentifier) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// InitRepo provides a mock function with given fields: repo
func (_m *Manager) InitRepo(repo *v1.BackupRepository) error {
	ret := _m.Called(repo)

	var r0 error
	if rf, ok := ret.Get(0).(func(*v1.BackupRepository) error); ok {
		r0 = rf(repo)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// PrepareRepo provides a mock function with given fields: repo
func (_m *Manager) PrepareRepo(repo *v1.BackupRepository) error {
	ret := _m.Called(repo)

	var r0 error
	if rf, ok := ret.Get(0).(func(*v1.BackupRepository) error); ok {
		r0 = rf(repo)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// PruneRepo provides a mock function with given fields: repo
func (_m *Manager) PruneRepo(repo *v1.BackupRepository) error {
	ret := _m.Called(repo)

	var r0 error
	if rf, ok := ret.Get(0).(func(*v1.BackupRepository) error); ok {
		r0 = rf(repo)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UnlockRepo provides a mock function with given fields: repo
func (_m *Manager) UnlockRepo(repo *v1.BackupRepository) error {
	ret := _m.Called(repo)

	var r0 error
	if rf, ok := ret.Get(0).(func(*v1.BackupRepository) error); ok {
		r0 = rf(repo)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewManager interface {
	mock.TestingT
	Cleanup(func())
}

// NewManager creates a new instance of Manager. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewManager(t mockConstructorTestingTNewManager) *Manager {
	mock := &Manager{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
