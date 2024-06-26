// Code generated by mockery v2.36.0. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"

// TransformerI is an autogenerated mock type for the TransformerI type
type TransformerI[T interface{}] struct {
	mock.Mock
}

type TransformerI_Expecter[T interface{}] struct {
	mock *mock.Mock
}

func (_m *TransformerI[T]) EXPECT() *TransformerI_Expecter[T] {
	return &TransformerI_Expecter[T]{mock: &_m.Mock}
}

// BytesToValue provides a mock function with given fields: _a0
func (_m *TransformerI[T]) BytesToValue(_a0 []byte) (T, error) {
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

// TransformerI_BytesToValue_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'BytesToValue'
type TransformerI_BytesToValue_Call[T interface{}] struct {
	*mock.Call
}

// BytesToValue is a helper method to define mock.On call
//   - _a0 []byte
func (_e *TransformerI_Expecter[T]) BytesToValue(_a0 interface{}) *TransformerI_BytesToValue_Call[T] {
	return &TransformerI_BytesToValue_Call[T]{Call: _e.mock.On("BytesToValue", _a0)}
}

func (_c *TransformerI_BytesToValue_Call[T]) Run(run func(_a0 []byte)) *TransformerI_BytesToValue_Call[T] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].([]byte))
	})
	return _c
}

func (_c *TransformerI_BytesToValue_Call[T]) Return(_a0 T, _a1 error) *TransformerI_BytesToValue_Call[T] {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *TransformerI_BytesToValue_Call[T]) RunAndReturn(run func([]byte) (T, error)) *TransformerI_BytesToValue_Call[T] {
	_c.Call.Return(run)
	return _c
}

// ValueToBytes provides a mock function with given fields: _a0
func (_m *TransformerI[T]) ValueToBytes(_a0 T) ([]byte, error) {
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

// TransformerI_ValueToBytes_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ValueToBytes'
type TransformerI_ValueToBytes_Call[T interface{}] struct {
	*mock.Call
}

// ValueToBytes is a helper method to define mock.On call
//   - _a0 T
func (_e *TransformerI_Expecter[T]) ValueToBytes(_a0 interface{}) *TransformerI_ValueToBytes_Call[T] {
	return &TransformerI_ValueToBytes_Call[T]{Call: _e.mock.On("ValueToBytes", _a0)}
}

func (_c *TransformerI_ValueToBytes_Call[T]) Run(run func(_a0 T)) *TransformerI_ValueToBytes_Call[T] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(T))
	})
	return _c
}

func (_c *TransformerI_ValueToBytes_Call[T]) Return(_a0 []byte, _a1 error) *TransformerI_ValueToBytes_Call[T] {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *TransformerI_ValueToBytes_Call[T]) RunAndReturn(run func(T) ([]byte, error)) *TransformerI_ValueToBytes_Call[T] {
	_c.Call.Return(run)
	return _c
}

// NewTransformerI creates a new instance of TransformerI. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewTransformerI[T interface{}](t interface {
	mock.TestingT
	Cleanup(func())
}) *TransformerI[T] {
	mock := &TransformerI[T]{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
