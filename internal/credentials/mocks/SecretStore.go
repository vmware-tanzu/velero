// Code generated by mockery v2.14.0. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
)

// SecretStore is an autogenerated mock type for the SecretStore type
type SecretStore struct {
	mock.Mock
}

// Get provides a mock function with given fields: selector
func (_m *SecretStore) Get(selector *v1.SecretKeySelector) (string, error) {
	ret := _m.Called(selector)

	var r0 string
	if rf, ok := ret.Get(0).(func(*v1.SecretKeySelector) string); ok {
		r0 = rf(selector)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*v1.SecretKeySelector) error); ok {
		r1 = rf(selector)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewSecretStore interface {
	mock.TestingT
	Cleanup(func())
}

// NewSecretStore creates a new instance of SecretStore. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewSecretStore(t mockConstructorTestingTNewSecretStore) *SecretStore {
	mock := &SecretStore{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
