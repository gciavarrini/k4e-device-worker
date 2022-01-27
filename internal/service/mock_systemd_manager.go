// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/jakub-dzon/k4e-device-worker/internal/service (interfaces: SystemdManager)

// Package service is a generated GoMock package.
package service

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockSystemdManager is a mock of SystemdManager interface.
type MockSystemdManager struct {
	ctrl     *gomock.Controller
	recorder *MockSystemdManagerMockRecorder
}

// MockSystemdManagerMockRecorder is the mock recorder for MockSystemdManager.
type MockSystemdManagerMockRecorder struct {
	mock *MockSystemdManager
}

// NewMockSystemdManager creates a new mock instance.
func NewMockSystemdManager(ctrl *gomock.Controller) *MockSystemdManager {
	mock := &MockSystemdManager{ctrl: ctrl}
	mock.recorder = &MockSystemdManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSystemdManager) EXPECT() *MockSystemdManagerMockRecorder {
	return m.recorder
}

// Add mocks base method.
func (m *MockSystemdManager) Add(arg0 Service) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Add", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Add indicates an expected call of Add.
func (mr *MockSystemdManagerMockRecorder) Add(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Add", reflect.TypeOf((*MockSystemdManager)(nil).Add), arg0)
}

// Get mocks base method.
func (m *MockSystemdManager) Get(arg0 string) Service {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0)
	ret0, _ := ret[0].(Service)
	return ret0
}

// Get indicates an expected call of Get.
func (mr *MockSystemdManagerMockRecorder) Get(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockSystemdManager)(nil).Get), arg0)
}

// Remove mocks base method.
func (m *MockSystemdManager) Remove(arg0 Service) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Remove", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Remove indicates an expected call of Remove.
func (mr *MockSystemdManagerMockRecorder) Remove(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Remove", reflect.TypeOf((*MockSystemdManager)(nil).Remove), arg0)
}