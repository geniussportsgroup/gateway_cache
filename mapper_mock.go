// Code generated by mockery v2.20.0. DO NOT EDIT.

package gw_cache

import mock "github.com/stretchr/testify/mock"

// MockMapperI is an autogenerated mock type for the MapperI type
type MockMapperI[T interface{}] struct {
	mock.Mock
}

type MockMapperI_Expecter[T interface{}] struct {
	mock *mock.Mock
}

func (_m *MockMapperI[T]) EXPECT() *MockMapperI_Expecter[T] {
	return &MockMapperI_Expecter[T]{mock: &_m.Mock}
}

// ToMapKey provides a mock function with given fields: _a0
func (_m *MockMapperI[T]) ToMapKey(_a0 T) (string, error) {
	ret := _m.Called(_a0)

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func(T) (string, error)); ok {
		return rf(_a0)
	}
	if rf, ok := ret.Get(0).(func(T) string); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(T) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockMapperI_ToMapKey_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ToMapKey'
type MockMapperI_ToMapKey_Call[T interface{}] struct {
	*mock.Call
}

// ToMapKey is a helper method to define mock.On call
//   - _a0 T
func (_e *MockMapperI_Expecter[T]) ToMapKey(_a0 interface{}) *MockMapperI_ToMapKey_Call[T] {
	return &MockMapperI_ToMapKey_Call[T]{Call: _e.mock.On("ToMapKey", _a0)}
}

func (_c *MockMapperI_ToMapKey_Call[T]) Run(run func(_a0 T)) *MockMapperI_ToMapKey_Call[T] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(T))
	})
	return _c
}

func (_c *MockMapperI_ToMapKey_Call[T]) Return(_a0 string, _a1 error) *MockMapperI_ToMapKey_Call[T] {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockMapperI_ToMapKey_Call[T]) RunAndReturn(run func(T) (string, error)) *MockMapperI_ToMapKey_Call[T] {
	_c.Call.Return(run)
	return _c
}

type mockConstructorTestingTNewMockMapperI interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockMapperI creates a new instance of MockMapperI. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockMapperI[T interface{}](t mockConstructorTestingTNewMockMapperI) *MockMapperI[T] {
	mock := &MockMapperI[T]{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
