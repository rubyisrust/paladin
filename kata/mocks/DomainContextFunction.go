// Code generated by mockery v2.43.2. DO NOT EDIT.

package mocks

import (
	context "context"

	statestore "github.com/kaleido-io/paladin/kata/internal/statestore"
	mock "github.com/stretchr/testify/mock"
)

// DomainContextFunction is an autogenerated mock type for the DomainContextFunction type
type DomainContextFunction struct {
	mock.Mock
}

// Execute provides a mock function with given fields: ctx, dsi
func (_m *DomainContextFunction) Execute(ctx context.Context, dsi statestore.DomainStateInterface) error {
	ret := _m.Called(ctx, dsi)

	if len(ret) == 0 {
		panic("no return value specified for Execute")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, statestore.DomainStateInterface) error); ok {
		r0 = rf(ctx, dsi)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewDomainContextFunction creates a new instance of DomainContextFunction. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewDomainContextFunction(t interface {
	mock.TestingT
	Cleanup(func())
}) *DomainContextFunction {
	mock := &DomainContextFunction{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
