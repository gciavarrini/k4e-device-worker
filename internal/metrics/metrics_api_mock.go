// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/jakub-dzon/k4e-device-worker/internal/metrics (interfaces: API)

// Package metrics is a generated GoMock package.
package metrics

import (
	reflect "reflect"
	time "time"

	gomock "github.com/golang/mock/gomock"
)

// MockAPI is a mock of API interface.
type MockAPI struct {
	ctrl     *gomock.Controller
	recorder *MockAPIMockRecorder
}

// MockAPIMockRecorder is the mock recorder for MockAPI.
type MockAPIMockRecorder struct {
	mock *MockAPI
}

// NewMockAPI creates a new mock instance.
func NewMockAPI(ctrl *gomock.Controller) *MockAPI {
	mock := &MockAPI{ctrl: ctrl}
	mock.recorder = &MockAPIMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAPI) EXPECT() *MockAPIMockRecorder {
	return m.recorder
}

// AddMetric mocks base method.
func (m *MockAPI) AddMetric(arg0 float64, arg1 map[string]string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddMetric", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddMetric indicates an expected call of AddMetric.
func (mr *MockAPIMockRecorder) AddMetric(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddMetric", reflect.TypeOf((*MockAPI)(nil).AddMetric), arg0, arg1)
}

// Close mocks base method.
func (m *MockAPI) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockAPIMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockAPI)(nil).Close))
}

// Deregister mocks base method.
func (m *MockAPI) Deregister() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Deregister")
	ret0, _ := ret[0].(error)
	return ret0
}

// Deregister indicates an expected call of Deregister.
func (mr *MockAPIMockRecorder) Deregister() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Deregister", reflect.TypeOf((*MockAPI)(nil).Deregister))
}

// GetMetricsForTimeRange mocks base method.
func (m *MockAPI) GetMetricsForTimeRange(arg0, arg1 time.Time) ([]Series, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMetricsForTimeRange", arg0, arg1)
	ret0, _ := ret[0].([]Series)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetMetricsForTimeRange indicates an expected call of GetMetricsForTimeRange.
func (mr *MockAPIMockRecorder) GetMetricsForTimeRange(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMetricsForTimeRange", reflect.TypeOf((*MockAPI)(nil).GetMetricsForTimeRange), arg0, arg1)
}
