// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/jakub-dzon/k4e-device-worker/internal/registration (interfaces: DispatcherClient)

// Package registration is a generated GoMock package.
package registration

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	protocol "github.com/redhatinsights/yggdrasil/protocol"
	grpc "google.golang.org/grpc"
)

// MockDispatcherClient is a mock of DispatcherClient interface.
type MockDispatcherClient struct {
	ctrl     *gomock.Controller
	recorder *MockDispatcherClientMockRecorder
}

// MockDispatcherClientMockRecorder is the mock recorder for MockDispatcherClient.
type MockDispatcherClientMockRecorder struct {
	mock *MockDispatcherClient
}

// NewMockDispatcherClient creates a new mock instance.
func NewMockDispatcherClient(ctrl *gomock.Controller) *MockDispatcherClient {
	mock := &MockDispatcherClient{ctrl: ctrl}
	mock.recorder = &MockDispatcherClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDispatcherClient) EXPECT() *MockDispatcherClientMockRecorder {
	return m.recorder
}

// Register mocks base method.
func (m *MockDispatcherClient) Register(arg0 context.Context, arg1 *protocol.RegistrationRequest, arg2 ...grpc.CallOption) (*protocol.RegistrationResponse, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Register", varargs...)
	ret0, _ := ret[0].(*protocol.RegistrationResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Register indicates an expected call of Register.
func (mr *MockDispatcherClientMockRecorder) Register(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Register", reflect.TypeOf((*MockDispatcherClient)(nil).Register), varargs...)
}

// Send mocks base method.
func (m *MockDispatcherClient) Send(arg0 context.Context, arg1 *protocol.Data, arg2 ...grpc.CallOption) (*protocol.Receipt, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Send", varargs...)
	ret0, _ := ret[0].(*protocol.Receipt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Send indicates an expected call of Send.
func (mr *MockDispatcherClientMockRecorder) Send(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Send", reflect.TypeOf((*MockDispatcherClient)(nil).Send), varargs...)
}