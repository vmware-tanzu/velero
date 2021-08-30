// Code generated by mockery v2.1.0. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"
	velero "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// PreRestoreAction is an autogenerated mock type for the PreRestoreAction type
type PreRestoreAction struct {
	mock.Mock
}

// Execute provides a mock function with given fields: input
func (_m *PreRestoreAction) Execute(input *velero.Restore) error {
	ret := _m.Called(input)

	var r0 error
	if rf, ok := ret.Get(0).(func(*velero.Restore) error); ok {
		r0 = rf(input)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
