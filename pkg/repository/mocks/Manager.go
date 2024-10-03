// Code generated by mockery v2.39.1. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	time "time"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// Manager is an autogenerated mock type for the Manager type
type Manager struct {
	mock.Mock
}

// BatchForget provides a mock function with given fields: _a0, _a1, _a2
func (_m *Manager) BatchForget(_a0 context.Context, _a1 *v1.BackupRepository, _a2 []string) []error {
	ret := _m.Called(_a0, _a1, _a2)

	if len(ret) == 0 {
		panic("no return value specified for BatchForget")
	}

	var r0 []error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.BackupRepository, []string) []error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]error)
		}
	}

	return r0
}

// ConnectToRepo provides a mock function with given fields: repo
func (_m *Manager) ConnectToRepo(repo *v1.BackupRepository) error {
	ret := _m.Called(repo)

	if len(ret) == 0 {
		panic("no return value specified for ConnectToRepo")
	}

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

	if len(ret) == 0 {
		panic("no return value specified for DefaultMaintenanceFrequency")
	}

	var r0 time.Duration
	var r1 error
	if rf, ok := ret.Get(0).(func(*v1.BackupRepository) (time.Duration, error)); ok {
		return rf(repo)
	}
	if rf, ok := ret.Get(0).(func(*v1.BackupRepository) time.Duration); ok {
		r0 = rf(repo)
	} else {
		r0 = ret.Get(0).(time.Duration)
	}

	if rf, ok := ret.Get(1).(func(*v1.BackupRepository) error); ok {
		r1 = rf(repo)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Forget provides a mock function with given fields: _a0, _a1, _a2
func (_m *Manager) Forget(_a0 context.Context, _a1 *v1.BackupRepository, _a2 string) error {
	ret := _m.Called(_a0, _a1, _a2)

	if len(ret) == 0 {
		panic("no return value specified for Forget")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.BackupRepository, string) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// InitRepo provides a mock function with given fields: repo
func (_m *Manager) InitRepo(repo *v1.BackupRepository) error {
	ret := _m.Called(repo)

	if len(ret) == 0 {
		panic("no return value specified for InitRepo")
	}

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

	if len(ret) == 0 {
		panic("no return value specified for PrepareRepo")
	}

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

	if len(ret) == 0 {
		panic("no return value specified for PruneRepo")
	}

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

	if len(ret) == 0 {
		panic("no return value specified for UnlockRepo")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(*v1.BackupRepository) error); ok {
		r0 = rf(repo)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewManager creates a new instance of Manager. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewManager(t interface {
	mock.TestingT
	Cleanup(func())
}) *Manager {
	mock := &Manager{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
