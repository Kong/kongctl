package config

import (
	"fmt"
	"os"

	"github.com/kong/kong-cli/internal/cmd/common"
	"github.com/kong/kong-cli/internal/meta"
	"github.com/kong/kong-cli/internal/util/viper"
	"github.com/spf13/pflag"
	v "github.com/spf13/viper"
)

var (
	defaultConfigPath     = "$XDG_CONFIG_HOME/" + meta.CLIName
	defaultConfigFilePath = defaultConfigPath + "/config.yaml"
)

var OuptutFormat = common.DefaultOutputFormat

func ExpandDefaultConfigPath() string {
	return os.ExpandEnv(defaultConfigPath)
}

func ExpandDefaultConfigFilePath() string {
	return os.ExpandEnv(defaultConfigFilePath)
}

// GetConfig returns the configuration for this instance of the CLI
func GetConfig(path string, profile string) (*ProfiledConfig, error) {
	var rv *ProfiledConfig
	var err error

	path = os.ExpandEnv(path)
	_, err = os.Stat(path)
	if err == nil {
		// If the user provides a file path, we should strictly load it or fail immediately
		vip, e := viper.NewViperE(path)
		if e == nil {
			rv = BuildProfiledConfig(profile, vip)
		} else {
			err = e
		}
	} else if path == defaultConfigFilePath {
		// TODO: There may be other cases where err != nil but we don't want to initialize the default
		vip, e := viper.InitializeDefaultViper(getDefaultConfig(profile), path)
		if e == nil {
			rv = BuildProfiledConfig(profile, vip)
		} else {
			err = e
		}
	} else {
		err = fmt.Errorf("the provided config file path does not exist")
	}
	return rv, err
}

// Empty type to represent the _type_ Config. Genesis is to support a key in a Context
type Key struct{}

// Config is a global instance of the Key type
var ConfigKey = Key{}

// Hook provides a generatlization of the Viper interface
// but allows some control, specifically over the Save functionality
// which we extend to provide safer file management handling
type Hook interface {
	// Save writes the configuration to the file system
	// TODO: Evaluate if writing the credentials is something we want to do at all
	//   I saw some issues related to writing config which may have been loaded by a variety of sources
	//   and is that desirable behavior (also security concerns with secrets loaded at runtime)
	Save() error
	// GetString returns a string value from the configuration
	GetString(key string) string
	// GetBool returns a boolean value from the configuration
	GetBool(key string) bool
	// GetInt returns an integer value from the configuration
	GetInt(key string) int
	// GetStringSlice returns a slice of strings from the configuration
	GetStringSlice(key string) []string
	// SetString sets an override for a given string
	SetString(key string, value string)
	// Set sets an override for a given key
	Set(k string, v any)
	// Get returns a value from the configuration
	Get(key string) any
	// BindFlag takes a specific configuration path and
	// binds it to a specific flag
	BindFlag(configPath string, f *pflag.Flag) error
	// The profile for this configuration
	GetProfile() string
}

// ProfiledConfig is a Viper but with an associated profile ProfileName
//
//	allows for extraction of the profile specific sub-configuration
//	and implements the Hook interface for more restricted interactions
//	with the configuration system
type ProfiledConfig struct {
	*v.Viper
	subViper    *v.Viper
	ProfileName string
}

func (p *ProfiledConfig) GetProfile() string {
	return p.ProfileName
}

func (p *ProfiledConfig) Save() error {
	// For now just defer to the write, but we want to add
	// file backups and better handling here to protect
	// user data
	// TODO: Improve / Evaluate writing of configs (if at all)
	return p.Viper.WriteConfig()
}

func (p *ProfiledConfig) GetString(key string) string {
	return p.subViper.GetString(key)
}

func (p *ProfiledConfig) GetBool(key string) bool {
	return p.subViper.GetBool(key)
}

func (p *ProfiledConfig) GetInt(key string) int {
	return p.subViper.GetInt(key)
}

func (p *ProfiledConfig) GetStringSlice(key string) []string {
	return p.subViper.GetStringSlice(key)
}

func (p *ProfiledConfig) BindFlag(configPath string, f *pflag.Flag) error {
	return p.subViper.BindPFlag(configPath, f)
}

func (p *ProfiledConfig) SetString(k string, v string) {
	p.subViper.Set(k, v)
}

func (p *ProfiledConfig) Set(k string, v any) {
	p.subViper.Set(k, v)
}

func BuildProfiledConfig(profile string, v *v.Viper) *ProfiledConfig {
	rv := &ProfiledConfig{
		Viper:       v,
		ProfileName: profile,
		subViper:    v.Sub(profile),
	}
	return rv
}

func getDefaultConfig(profileName string) map[string]interface{} {
	defaultConfig := map[string]interface{}{
		profileName: map[string]interface{}{
			"output":  "text",
			"konnect": map[string]interface{}{},
		},
	}
	return defaultConfig
}
