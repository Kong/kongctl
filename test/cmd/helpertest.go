package cmd

import (
	"context"
	"log/slog"

	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
)

type MockHelper struct {
	GetCmdMock          func() *cobra.Command
	GetArgsMock         func() []string
	GetVerbMock         func() (verbs.VerbValue, error)
	GetProductMock      func() (products.ProductValue, error)
	GetStreamsMock      func() *iostreams.IOStreams
	GetConfigMock       func() (config.Hook, error)
	GetOutputFormatMock func() (common.OutputFormat, error)
	GetLoggerMock       func() (*slog.Logger, error)
	GetBuildInfoMock    func() (*build.Info, error)
	GetContextMock      func() context.Context
	GetKonnectSDKMock   func(config.Hook, *slog.Logger) (helpers.SDKAPI, error)
}

func (m *MockHelper) GetCmd() *cobra.Command {
	return m.GetCmdMock()
}

func (m *MockHelper) GetArgs() []string {
	return m.GetArgsMock()
}

func (m *MockHelper) GetVerb() (verbs.VerbValue, error) {
	return m.GetVerbMock()
}

func (m *MockHelper) GetProduct() (products.ProductValue, error) {
	return m.GetProductMock()
}

func (m *MockHelper) GetStreams() *iostreams.IOStreams {
	return m.GetStreamsMock()
}

func (m *MockHelper) GetConfig() (config.Hook, error) {
	return m.GetConfigMock()
}

func (m *MockHelper) GetKonnectSDK(cfg config.Hook, logger *slog.Logger) (helpers.SDKAPI, error) {
	return m.GetKonnectSDKMock(cfg, logger)
}

func (m *MockHelper) GetOutputFormat() (common.OutputFormat, error) {
	return m.GetOutputFormatMock()
}

func (m *MockHelper) GetLogger() (*slog.Logger, error) {
	return m.GetLoggerMock()
}

func (m *MockHelper) GetBuildInfo() (*build.Info, error) {
	return m.GetBuildInfoMock()
}

func (m *MockHelper) GetContext() context.Context {
	return m.GetContextMock()
}
