// Code generated by mockery v2.36.0. DO NOT EDIT.

package gw_cache

import mock "github.com/stretchr/testify/mock"

// MockProcessorI is an autogenerated mock type for the ProcessorI type
type MockProcessorI[K interface{}, T interface{}] struct {
	mock.Mock
}

type MockProcessorI_Expecter[K interface{}, T interface{}] struct {
	mock *mock.Mock
}

func (_m *MockProcessorI[K, T]) EXPECT() *MockProcessorI_Expecter[K, T] {
	return &MockProcessorI_Expecter[K, T]{mock: &_m.Mock}
}

// CacheMissSolver provides a mock function with given fields: _a0, _a1
func (_m *MockProcessorI[K, T]) CacheMissSolver(_a0 K, _a1 ...interface{}) (T, *RequestError) {
	var _ca []interface{}
	_ca = append(_ca, _a0)
	_ca = append(_ca, _a1...)
	ret := _m.Called(_ca...)

	var r0 T
	var r1 *RequestError
	if rf, ok := ret.Get(0).(func(K, ...interface{}) (T, *RequestError)); ok {
		return rf(_a0, _a1...)
	}
	if rf, ok := ret.Get(0).(func(K, ...interface{}) T); ok {
		r0 = rf(_a0, _a1...)
	} else {
		r0 = ret.Get(0).(T)
	}

	if rf, ok := ret.Get(1).(func(K, ...interface{}) *RequestError); ok {
		r1 = rf(_a0, _a1...)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*RequestError)
		}
	}

	return r0, r1
}

// MockProcessorI_CacheMissSolver_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CacheMissSolver'
type MockProcessorI_CacheMissSolver_Call[K interface{}, T interface{}] struct {
	*mock.Call
}

// CacheMissSolver is a helper method to define mock.On call
//   - _a0 K
//   - _a1 ...interface{}
func (_e *MockProcessorI_Expecter[K, T]) CacheMissSolver(_a0 interface{}, _a1 ...interface{}) *MockProcessorI_CacheMissSolver_Call[K, T] {
	return &MockProcessorI_CacheMissSolver_Call[K, T]{Call: _e.mock.On("CacheMissSolver",
		append([]interface{}{_a0}, _a1...)...)}
}

func (_c *MockProcessorI_CacheMissSolver_Call[K, T]) Run(run func(_a0 K, _a1 ...interface{})) *MockProcessorI_CacheMissSolver_Call[K, T] {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]interface{}, len(args)-1)
		for i, a := range args[1:] {
			if a != nil {
				variadicArgs[i] = a.(interface{})
			}
		}
		run(args[0].(K), variadicArgs...)
	})
	return _c
}

func (_c *MockProcessorI_CacheMissSolver_Call[K, T]) Return(_a0 T, _a1 *RequestError) *MockProcessorI_CacheMissSolver_Call[K, T] {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockProcessorI_CacheMissSolver_Call[K, T]) RunAndReturn(run func(K, ...interface{}) (T, *RequestError)) *MockProcessorI_CacheMissSolver_Call[K, T] {
	_c.Call.Return(run)
	return _c
}

// ToMapKey provides a mock function with given fields: _a0
func (_m *MockProcessorI[K, T]) ToMapKey(_a0 K) (string, error) {
	ret := _m.Called(_a0)

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func(K) (string, error)); ok {
		return rf(_a0)
	}
	if rf, ok := ret.Get(0).(func(K) string); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(K) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockProcessorI_ToMapKey_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ToMapKey'
type MockProcessorI_ToMapKey_Call[K interface{}, T interface{}] struct {
	*mock.Call
}

// ToMapKey is a helper method to define mock.On call
//   - _a0 K
func (_e *MockProcessorI_Expecter[K, T]) ToMapKey(_a0 interface{}) *MockProcessorI_ToMapKey_Call[K, T] {
	return &MockProcessorI_ToMapKey_Call[K, T]{Call: _e.mock.On("ToMapKey", _a0)}
}

func (_c *MockProcessorI_ToMapKey_Call[K, T]) Run(run func(_a0 K)) *MockProcessorI_ToMapKey_Call[K, T] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(K))
	})
	return _c
}

func (_c *MockProcessorI_ToMapKey_Call[K, T]) Return(_a0 string, _a1 error) *MockProcessorI_ToMapKey_Call[K, T] {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockProcessorI_ToMapKey_Call[K, T]) RunAndReturn(run func(K) (string, error)) *MockProcessorI_ToMapKey_Call[K, T] {
	_c.Call.Return(run)
	return _c
}

// NewMockProcessorI creates a new instance of MockProcessorI. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockProcessorI[K interface{}, T interface{}](t interface {
	mock.TestingT
	Cleanup(func())
}) *MockProcessorI[K, T] {
	mock := &MockProcessorI[K, T]{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}