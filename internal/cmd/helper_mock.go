// Code generated by mockery. DO NOT EDIT.

package cmd

import (
	build "github.com/kong/kongctl/internal/build"
	cobra "github.com/spf13/cobra"

	common "github.com/kong/kongctl/internal/cmd/common"

	config "github.com/kong/kongctl/internal/config"

	context "context"

	helpers "github.com/kong/kongctl/internal/konnect/helpers"

	iostreams "github.com/kong/kongctl/internal/iostreams"

	mock "github.com/stretchr/testify/mock"

	products "github.com/kong/kongctl/internal/cmd/root/products"

	slog "log/slog"

	verbs "github.com/kong/kongctl/internal/cmd/root/verbs"
)

// MockHelper is an autogenerated mock type for the Helper type
type MockHelper struct {
	mock.Mock
}

type MockHelper_Expecter struct {
	mock *mock.Mock
}

func (_m *MockHelper) EXPECT() *MockHelper_Expecter {
	return &MockHelper_Expecter{mock: &_m.Mock}
}

// GetArgs provides a mock function with given fields:
func (_m *MockHelper) GetArgs() []string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetArgs")
	}

	var r0 []string
	if rf, ok := ret.Get(0).(func() []string); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	return r0
}

// MockHelper_GetArgs_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetArgs'
type MockHelper_GetArgs_Call struct {
	*mock.Call
}

// GetArgs is a helper method to define mock.On call
func (_e *MockHelper_Expecter) GetArgs() *MockHelper_GetArgs_Call {
	return &MockHelper_GetArgs_Call{Call: _e.mock.On("GetArgs")}
}

func (_c *MockHelper_GetArgs_Call) Run(run func()) *MockHelper_GetArgs_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockHelper_GetArgs_Call) Return(_a0 []string) *MockHelper_GetArgs_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockHelper_GetArgs_Call) RunAndReturn(run func() []string) *MockHelper_GetArgs_Call {
	_c.Call.Return(run)
	return _c
}

// GetBuildInfo provides a mock function with given fields:
func (_m *MockHelper) GetBuildInfo() (*build.Info, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetBuildInfo")
	}

	var r0 *build.Info
	var r1 error
	if rf, ok := ret.Get(0).(func() (*build.Info, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() *build.Info); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*build.Info)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockHelper_GetBuildInfo_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetBuildInfo'
type MockHelper_GetBuildInfo_Call struct {
	*mock.Call
}

// GetBuildInfo is a helper method to define mock.On call
func (_e *MockHelper_Expecter) GetBuildInfo() *MockHelper_GetBuildInfo_Call {
	return &MockHelper_GetBuildInfo_Call{Call: _e.mock.On("GetBuildInfo")}
}

func (_c *MockHelper_GetBuildInfo_Call) Run(run func()) *MockHelper_GetBuildInfo_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockHelper_GetBuildInfo_Call) Return(_a0 *build.Info, _a1 error) *MockHelper_GetBuildInfo_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockHelper_GetBuildInfo_Call) RunAndReturn(run func() (*build.Info, error)) *MockHelper_GetBuildInfo_Call {
	_c.Call.Return(run)
	return _c
}

// GetCmd provides a mock function with given fields:
func (_m *MockHelper) GetCmd() *cobra.Command {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetCmd")
	}

	var r0 *cobra.Command
	if rf, ok := ret.Get(0).(func() *cobra.Command); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*cobra.Command)
		}
	}

	return r0
}

// MockHelper_GetCmd_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetCmd'
type MockHelper_GetCmd_Call struct {
	*mock.Call
}

// GetCmd is a helper method to define mock.On call
func (_e *MockHelper_Expecter) GetCmd() *MockHelper_GetCmd_Call {
	return &MockHelper_GetCmd_Call{Call: _e.mock.On("GetCmd")}
}

func (_c *MockHelper_GetCmd_Call) Run(run func()) *MockHelper_GetCmd_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockHelper_GetCmd_Call) Return(_a0 *cobra.Command) *MockHelper_GetCmd_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockHelper_GetCmd_Call) RunAndReturn(run func() *cobra.Command) *MockHelper_GetCmd_Call {
	_c.Call.Return(run)
	return _c
}

// GetConfig provides a mock function with given fields:
func (_m *MockHelper) GetConfig() (config.Hook, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetConfig")
	}

	var r0 config.Hook
	var r1 error
	if rf, ok := ret.Get(0).(func() (config.Hook, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() config.Hook); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(config.Hook)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockHelper_GetConfig_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetConfig'
type MockHelper_GetConfig_Call struct {
	*mock.Call
}

// GetConfig is a helper method to define mock.On call
func (_e *MockHelper_Expecter) GetConfig() *MockHelper_GetConfig_Call {
	return &MockHelper_GetConfig_Call{Call: _e.mock.On("GetConfig")}
}

func (_c *MockHelper_GetConfig_Call) Run(run func()) *MockHelper_GetConfig_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockHelper_GetConfig_Call) Return(_a0 config.Hook, _a1 error) *MockHelper_GetConfig_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockHelper_GetConfig_Call) RunAndReturn(run func() (config.Hook, error)) *MockHelper_GetConfig_Call {
	_c.Call.Return(run)
	return _c
}

// GetContext provides a mock function with given fields:
func (_m *MockHelper) GetContext() context.Context {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetContext")
	}

	var r0 context.Context
	if rf, ok := ret.Get(0).(func() context.Context); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(context.Context)
		}
	}

	return r0
}

// MockHelper_GetContext_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetContext'
type MockHelper_GetContext_Call struct {
	*mock.Call
}

// GetContext is a helper method to define mock.On call
func (_e *MockHelper_Expecter) GetContext() *MockHelper_GetContext_Call {
	return &MockHelper_GetContext_Call{Call: _e.mock.On("GetContext")}
}

func (_c *MockHelper_GetContext_Call) Run(run func()) *MockHelper_GetContext_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockHelper_GetContext_Call) Return(_a0 context.Context) *MockHelper_GetContext_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockHelper_GetContext_Call) RunAndReturn(run func() context.Context) *MockHelper_GetContext_Call {
	_c.Call.Return(run)
	return _c
}

// GetKonnectSDKFactory provides a mock function with given fields:
func (_m *MockHelper) GetKonnectSDKFactory() helpers.SDKAPIFactory {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetKonnectSDKFactory")
	}

	var r0 helpers.SDKAPIFactory
	if rf, ok := ret.Get(0).(func() helpers.SDKAPIFactory); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(helpers.SDKAPIFactory)
		}
	}

	return r0
}

// MockHelper_GetKonnectSDKFactory_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetKonnectSDKFactory'
type MockHelper_GetKonnectSDKFactory_Call struct {
	*mock.Call
}

// GetKonnectSDKFactory is a helper method to define mock.On call
func (_e *MockHelper_Expecter) GetKonnectSDKFactory() *MockHelper_GetKonnectSDKFactory_Call {
	return &MockHelper_GetKonnectSDKFactory_Call{Call: _e.mock.On("GetKonnectSDKFactory")}
}

func (_c *MockHelper_GetKonnectSDKFactory_Call) Run(run func()) *MockHelper_GetKonnectSDKFactory_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockHelper_GetKonnectSDKFactory_Call) Return(_a0 helpers.SDKAPIFactory) *MockHelper_GetKonnectSDKFactory_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockHelper_GetKonnectSDKFactory_Call) RunAndReturn(run func() helpers.SDKAPIFactory) *MockHelper_GetKonnectSDKFactory_Call {
	_c.Call.Return(run)
	return _c
}

// GetLogger provides a mock function with given fields:
func (_m *MockHelper) GetLogger() (*slog.Logger, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetLogger")
	}

	var r0 *slog.Logger
	var r1 error
	if rf, ok := ret.Get(0).(func() (*slog.Logger, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() *slog.Logger); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*slog.Logger)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockHelper_GetLogger_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetLogger'
type MockHelper_GetLogger_Call struct {
	*mock.Call
}

// GetLogger is a helper method to define mock.On call
func (_e *MockHelper_Expecter) GetLogger() *MockHelper_GetLogger_Call {
	return &MockHelper_GetLogger_Call{Call: _e.mock.On("GetLogger")}
}

func (_c *MockHelper_GetLogger_Call) Run(run func()) *MockHelper_GetLogger_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockHelper_GetLogger_Call) Return(_a0 *slog.Logger, _a1 error) *MockHelper_GetLogger_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockHelper_GetLogger_Call) RunAndReturn(run func() (*slog.Logger, error)) *MockHelper_GetLogger_Call {
	_c.Call.Return(run)
	return _c
}

// GetOutputFormat provides a mock function with given fields:
func (_m *MockHelper) GetOutputFormat() (common.OutputFormat, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetOutputFormat")
	}

	var r0 common.OutputFormat
	var r1 error
	if rf, ok := ret.Get(0).(func() (common.OutputFormat, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() common.OutputFormat); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(common.OutputFormat)
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockHelper_GetOutputFormat_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetOutputFormat'
type MockHelper_GetOutputFormat_Call struct {
	*mock.Call
}

// GetOutputFormat is a helper method to define mock.On call
func (_e *MockHelper_Expecter) GetOutputFormat() *MockHelper_GetOutputFormat_Call {
	return &MockHelper_GetOutputFormat_Call{Call: _e.mock.On("GetOutputFormat")}
}

func (_c *MockHelper_GetOutputFormat_Call) Run(run func()) *MockHelper_GetOutputFormat_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockHelper_GetOutputFormat_Call) Return(_a0 common.OutputFormat, _a1 error) *MockHelper_GetOutputFormat_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockHelper_GetOutputFormat_Call) RunAndReturn(run func() (common.OutputFormat, error)) *MockHelper_GetOutputFormat_Call {
	_c.Call.Return(run)
	return _c
}

// GetProduct provides a mock function with given fields:
func (_m *MockHelper) GetProduct() (products.ProductValue, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetProduct")
	}

	var r0 products.ProductValue
	var r1 error
	if rf, ok := ret.Get(0).(func() (products.ProductValue, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() products.ProductValue); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(products.ProductValue)
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockHelper_GetProduct_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetProduct'
type MockHelper_GetProduct_Call struct {
	*mock.Call
}

// GetProduct is a helper method to define mock.On call
func (_e *MockHelper_Expecter) GetProduct() *MockHelper_GetProduct_Call {
	return &MockHelper_GetProduct_Call{Call: _e.mock.On("GetProduct")}
}

func (_c *MockHelper_GetProduct_Call) Run(run func()) *MockHelper_GetProduct_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockHelper_GetProduct_Call) Return(_a0 products.ProductValue, _a1 error) *MockHelper_GetProduct_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockHelper_GetProduct_Call) RunAndReturn(run func() (products.ProductValue, error)) *MockHelper_GetProduct_Call {
	_c.Call.Return(run)
	return _c
}

// GetStreams provides a mock function with given fields:
func (_m *MockHelper) GetStreams() *iostreams.IOStreams {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetStreams")
	}

	var r0 *iostreams.IOStreams
	if rf, ok := ret.Get(0).(func() *iostreams.IOStreams); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*iostreams.IOStreams)
		}
	}

	return r0
}

// MockHelper_GetStreams_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetStreams'
type MockHelper_GetStreams_Call struct {
	*mock.Call
}

// GetStreams is a helper method to define mock.On call
func (_e *MockHelper_Expecter) GetStreams() *MockHelper_GetStreams_Call {
	return &MockHelper_GetStreams_Call{Call: _e.mock.On("GetStreams")}
}

func (_c *MockHelper_GetStreams_Call) Run(run func()) *MockHelper_GetStreams_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockHelper_GetStreams_Call) Return(_a0 *iostreams.IOStreams) *MockHelper_GetStreams_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockHelper_GetStreams_Call) RunAndReturn(run func() *iostreams.IOStreams) *MockHelper_GetStreams_Call {
	_c.Call.Return(run)
	return _c
}

// GetVerb provides a mock function with given fields:
func (_m *MockHelper) GetVerb() (verbs.VerbValue, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetVerb")
	}

	var r0 verbs.VerbValue
	var r1 error
	if rf, ok := ret.Get(0).(func() (verbs.VerbValue, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() verbs.VerbValue); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(verbs.VerbValue)
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockHelper_GetVerb_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetVerb'
type MockHelper_GetVerb_Call struct {
	*mock.Call
}

// GetVerb is a helper method to define mock.On call
func (_e *MockHelper_Expecter) GetVerb() *MockHelper_GetVerb_Call {
	return &MockHelper_GetVerb_Call{Call: _e.mock.On("GetVerb")}
}

func (_c *MockHelper_GetVerb_Call) Run(run func()) *MockHelper_GetVerb_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockHelper_GetVerb_Call) Return(_a0 verbs.VerbValue, _a1 error) *MockHelper_GetVerb_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockHelper_GetVerb_Call) RunAndReturn(run func() (verbs.VerbValue, error)) *MockHelper_GetVerb_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockHelper creates a new instance of MockHelper. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockHelper(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockHelper {
	mock := &MockHelper{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
