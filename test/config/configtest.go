package config

import (
	"github.com/spf13/pflag"
)

type MockConfigHook struct {
	GetStringMock      func(key string) string
	GetBoolMock        func(key string) bool
	GetIntMock         func(key string) int
	GetIntOrElseMock   func(key string, orElse int) int
	SaveMock           func() error
	BindFlagMock       func(string, *pflag.Flag) error
	GetProfileMock     func() string
	GetStringSlickMock func(key string) []string
	SetStringMock      func(k string, v string)
	SetMock            func(k string, v any)
	GetMock            func(k string) any
	GetPathMock        func() string
}

func (m *MockConfigHook) Save() error {
	return m.SaveMock()
}

func (m *MockConfigHook) GetString(key string) string {
	return m.GetStringMock(key)
}

func (m *MockConfigHook) GetBool(key string) bool {
	return m.GetBoolMock(key)
}

func (m *MockConfigHook) GetInt(key string) int {
	return m.GetIntMock(key)
}

func (m *MockConfigHook) GetIntOrElse(key string, orElse int) int {
	if m.GetIntOrElseMock != nil {
		return m.GetIntOrElseMock(key, orElse)
	}
	return orElse
}

func (m *MockConfigHook) BindFlag(configPath string, f *pflag.Flag) error {
	return m.BindFlagMock(configPath, f)
}

func (m *MockConfigHook) GetProfile() string {
	return m.GetProfileMock()
}

func (m *MockConfigHook) GetStringSlice(key string) []string {
	return m.GetStringSlickMock(key)
}

func (m *MockConfigHook) SetString(k string, v string) {
	m.SetStringMock(k, v)
}

func (m *MockConfigHook) Set(k string, v any) {
	m.SetMock(k, v)
}

func (m *MockConfigHook) Get(k string) any {
	return m.GetMock(k)
}

func (m *MockConfigHook) GetPath() string {
	return m.GetPathMock()
}
