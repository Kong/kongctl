package cmd

import (
	"log/slog"

	"github.com/kong/kong-cli/internal/cmd/root/products"
	"github.com/kong/kong-cli/internal/cmd/root/verbs"
	"github.com/kong/kong-cli/internal/config"
	"github.com/kong/kong-cli/internal/iostreams"
	"github.com/spf13/cobra"
)

type MockHelper struct {
	GetCmdMock          func() *cobra.Command
	GetArgsMock         func() []string
	GetVerbMock         func() (verbs.VerbValue, error)
	GetProductMock      func() (products.ProductValue, error)
	GetStreamsMock      func() *iostreams.IOStreams
	GetConfigMock       func() (config.Hook, error)
	GetOutputFormatMock func() (string, error)
	GetLoggerMock       func() (*slog.Logger, error)
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

func (m *MockHelper) GetOutputFormat() (string, error) {
	return m.GetOutputFormatMock()
}

func (m *MockHelper) GetLogger() (*slog.Logger, error) {
	return m.GetLoggerMock()
}
