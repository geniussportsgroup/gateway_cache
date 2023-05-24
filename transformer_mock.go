// Code generated by mockery v2.20.0. DO NOT EDIT.

package gw_cache

import mock "github.com/stretchr/testify/mock"

// MockTransformerI is an autogenerated mock type for the TransformerI type
type MockTransformerI[T interface{}] struct {
	mock.Mock
}

type MockTransformerI_Expecter[T interface{}] struct {
	mock *mock.Mock
}

func (_m *MockTransformerI[T]) EXPECT() *MockTransformerI_Expecter[T] {
	return &MockTransformerI_Expecter[T]{mock: &_m.Mock}
}

// BytesToValue provides a mock function with given fields: _a0
func (_m *MockTransformerI[T]) BytesToValue(_a0 []byte) (T, error) {
	ret := _m.Called(_a0)

	var r0 T
	var r1 error
	if rf, ok := ret.Get(0).(func([]byte) (T, error)); ok {
		return rf(_a0)
	}
	if rf, ok := ret.Get(0).(func([]byte) T); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(T)
	}

	if rf, ok := ret.Get(1).(func([]byte) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockTransformerI_BytesToValue_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'BytesToValue'
type MockTransformerI_BytesToValue_Call[T interface{}] struct {
	*mock.Call
}

// BytesToValue is a helper method to define mock.On call
//   - _a0 []byte
func (_e *MockTransformerI_Expecter[T]) BytesToValue(_a0 interface{}) *MockTransformerI_BytesToValue_Call[T] {
	return &MockTransformerI_BytesToValue_Call[T]{Call: _e.mock.On("BytesToValue", _a0)}
}

func (_c *MockTransformerI_BytesToValue_Call[T]) Run(run func(_a0 []byte)) *MockTransformerI_BytesToValue_Call[T] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].([]byte))
	})
	return _c
}

func (_c *MockTransformerI_BytesToValue_Call[T]) Return(_a0 T, _a1 error) *MockTransformerI_BytesToValue_Call[T] {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockTransformerI_BytesToValue_Call[T]) RunAndReturn(run func([]byte) (T, error)) *MockTransformerI_BytesToValue_Call[T] {
	_c.Call.Return(run)
	return _c
}

// ValueToBytes provides a mock function with given fields: _a0
func (_m *MockTransformerI[T]) ValueToBytes(_a0 T) ([]byte, error) {
	ret := _m.Called(_a0)

	var r0 []byte
	var r1 error
	if rf, ok := ret.Get(0).(func(T) ([]byte, error)); ok {
		return rf(_a0)
	}
	if rf, ok := ret.Get(0).(func(T) []byte); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	if rf, ok := ret.Get(1).(func(T) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockTransformerI_ValueToBytes_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ValueToBytes'
type MockTransformerI_ValueToBytes_Call[T interface{}] struct {
	*mock.Call
}

// ValueToBytes is a helper method to define mock.On call
//   - _a0 T
func (_e *MockTransformerI_Expecter[T]) ValueToBytes(_a0 interface{}) *MockTransformerI_ValueToBytes_Call[T] {
	return &MockTransformerI_ValueToBytes_Call[T]{Call: _e.mock.On("ValueToBytes", _a0)}
}

func (_c *MockTransformerI_ValueToBytes_Call[T]) Run(run func(_a0 T)) *MockTransformerI_ValueToBytes_Call[T] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(T))
	})
	return _c
}

func (_c *MockTransformerI_ValueToBytes_Call[T]) Return(_a0 []byte, _a1 error) *MockTransformerI_ValueToBytes_Call[T] {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockTransformerI_ValueToBytes_Call[T]) RunAndReturn(run func(T) ([]byte, error)) *MockTransformerI_ValueToBytes_Call[T] {
	_c.Call.Return(run)
	return _c
}

type mockConstructorTestingTNewMockTransformerI interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockTransformerI creates a new instance of MockTransformerI. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockTransformerI[T interface{}](t mockConstructorTestingTNewMockTransformerI) *MockTransformerI[T] {
	mock := &MockTransformerI[T]{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
