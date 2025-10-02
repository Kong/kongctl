package adopt

import (
	"github.com/kong/kongctl/internal/config"
	"github.com/spf13/pflag"
)

type stubConfig struct {
	pageSize int
}

func (s stubConfig) Save() error                           { return nil }
func (s stubConfig) GetString(string) string               { return "" }
func (s stubConfig) GetBool(string) bool                   { return false }
func (s stubConfig) GetInt(string) int                     { return s.pageSize }
func (s stubConfig) GetIntOrElse(_ string, orElse int) int { return orElse }
func (s stubConfig) GetStringSlice(string) []string        { return nil }
func (s stubConfig) SetString(string, string)              {}
func (s stubConfig) Set(string, any)                       {}
func (s stubConfig) Get(string) any                        { return nil }
func (s stubConfig) BindFlag(string, *pflag.Flag) error    { return nil }
func (s stubConfig) GetProfile() string                    { return "default" }
func (s stubConfig) GetPath() string                       { return "" }

var _ config.Hook = stubConfig{}
