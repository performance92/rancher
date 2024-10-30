// Code generated by mockery v2.30.16. DO NOT EDIT.

package mocks

import (
	context "context"

	catalog_cattle_iov1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"

	io "io"

	mock "github.com/stretchr/testify/mock"

	user "k8s.io/apiserver/pkg/authentication/user"

	v1 "k8s.io/api/core/v1"
)

// OperationClient is an autogenerated mock type for the OperationClient type
type OperationClient struct {
	mock.Mock
}

// AddCpTaintsToTolerations provides a mock function with given fields: tolerations
func (_m *OperationClient) AddCpTaintsToTolerations(tolerations []v1.Toleration) ([]v1.Toleration, error) {
	ret := _m.Called(tolerations)

	var r0 []v1.Toleration
	var r1 error
	if rf, ok := ret.Get(0).(func([]v1.Toleration) ([]v1.Toleration, error)); ok {
		return rf(tolerations)
	}
	if rf, ok := ret.Get(0).(func([]v1.Toleration) []v1.Toleration); ok {
		r0 = rf(tolerations)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]v1.Toleration)
		}
	}

	if rf, ok := ret.Get(1).(func([]v1.Toleration) error); ok {
		r1 = rf(tolerations)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Uninstall provides a mock function with given fields: ctx, _a1, namespace, name, options, imageOverride
func (_m *OperationClient) Uninstall(ctx context.Context, _a1 user.Info, namespace string, name string, options io.Reader, imageOverride string) (*catalog_cattle_iov1.Operation, error) {
	ret := _m.Called(ctx, _a1, namespace, name, options, imageOverride)

	var r0 *catalog_cattle_iov1.Operation
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, user.Info, string, string, io.Reader, string) (*catalog_cattle_iov1.Operation, error)); ok {
		return rf(ctx, _a1, namespace, name, options, imageOverride)
	}
	if rf, ok := ret.Get(0).(func(context.Context, user.Info, string, string, io.Reader, string) *catalog_cattle_iov1.Operation); ok {
		r0 = rf(ctx, _a1, namespace, name, options, imageOverride)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*catalog_cattle_iov1.Operation)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, user.Info, string, string, io.Reader, string) error); ok {
		r1 = rf(ctx, _a1, namespace, name, options, imageOverride)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Upgrade provides a mock function with given fields: ctx, _a1, namespace, name, options, imageOverride
func (_m *OperationClient) Upgrade(ctx context.Context, _a1 user.Info, namespace string, name string, options io.Reader, imageOverride string) (*catalog_cattle_iov1.Operation, error) {
	ret := _m.Called(ctx, _a1, namespace, name, options, imageOverride)

	var r0 *catalog_cattle_iov1.Operation
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, user.Info, string, string, io.Reader, string) (*catalog_cattle_iov1.Operation, error)); ok {
		return rf(ctx, _a1, namespace, name, options, imageOverride)
	}
	if rf, ok := ret.Get(0).(func(context.Context, user.Info, string, string, io.Reader, string) *catalog_cattle_iov1.Operation); ok {
		r0 = rf(ctx, _a1, namespace, name, options, imageOverride)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*catalog_cattle_iov1.Operation)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, user.Info, string, string, io.Reader, string) error); ok {
		r1 = rf(ctx, _a1, namespace, name, options, imageOverride)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewOperationClient creates a new instance of OperationClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewOperationClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *OperationClient {
	mock := &OperationClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
