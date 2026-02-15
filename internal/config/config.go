package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/viper"
	"github.com/spf13/pflag"
	v "github.com/spf13/viper"
)

var defaultConfigFileName = "config.yaml"

var OuptutFormat = common.DefaultOutputFormat

// Returns the expanded default config path depending on what
// environment variables are set. If XDG_CONFIG_HOME is set,
// the default is $XDG_CONFIG_HOME/kongctl,
// otherwise the default is os.UserHomeDir()/.config/kongctl.
// If these values are not set, an error is returned.
func GetDefaultConfigPath() (string, error) {
	val, set := os.LookupEnv("XDG_CONFIG_HOME")
	if !set || val == "" {
		var err error
		val, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
		val = filepath.Join(val, ".config")
	}
	val = filepath.Join(val, meta.CLIName)
	return os.ExpandEnv(val), nil
}

func GetDefaultConfigFilePath() (string, error) {
	path, err := GetDefaultConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(path, defaultConfigFileName), nil
}

// GetConfig returns the configuration for this instance of the CLI
func GetConfig(path string, profile string, defaultConfigFilePath string) (*ProfiledConfig, error) {
	var rv *ProfiledConfig
	var err error

	path = os.ExpandEnv(path)

	_, err = os.Stat(path)
	if err == nil {
		// If the user provides a valid file path, we should strictly load it or fail immediately
		vip, e := viper.NewViperE(path)
		if e == nil {
			rv = BuildProfiledConfig(profile, path, vip)
		} else {
			err = e
		}
	} else if path == defaultConfigFilePath {
		// If the default given config file path does not exist, and it matches the defaultConfigFilePath
		// then we should initialize the default configuration including creating the directory and file
		var vip *v.Viper
		vip, err = viper.InitializeDefaultViper(getDefaultConfig(profile, path), path)
		if err == nil {
			rv = BuildProfiledConfig(profile, path, vip)
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
	// GetIntOrElse returns an integer value from the configuration or a default
	GetIntOrElse(key string, orElse int) int
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
	// The file path used to load this configuration
	GetPath() string
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
	Path        string
}

func (p *ProfiledConfig) GetProfile() string {
	return p.ProfileName
}

func (p *ProfiledConfig) Save() error {
	// For now just defer to the write, but we want to add
	// file backups and better handling here to protect
	// user data
	// TODO: Improve / Evaluate writing of configs (if at all)
	return p.WriteConfig()
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

func (p *ProfiledConfig) GetIntOrElse(key string, orElse int) int {
	if p.subViper.IsSet(key) {
		return p.subViper.GetInt(key)
	}
	return orElse
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

func (p *ProfiledConfig) GetPath() string {
	return p.Path
}

func BuildProfiledConfig(profile string, path string, mainv *v.Viper) *ProfiledConfig {
	subv := mainv.Sub(profile)
	if subv == nil {
		// in this case the main viper is valid, but there is no
		// key or data under the key for this profile name
		subv = v.New()
		// Configure environment variable handling for the new sub-Viper
		// so it can still read profile-specific environment variables
		// even when the profile doesn't exist in the config file
		envPrefix := "KONGCTL_" + strings.ToUpper(strings.ReplaceAll(profile, "-", "_"))
		viper.ConfigureEnvVars(subv, envPrefix)
	}

	rv := &ProfiledConfig{
		Viper:       mainv,
		ProfileName: profile,
		subViper:    subv,
		Path:        path,
	}
	return rv
}

func getDefaultConfig(profileName, configFilePath string) map[string]any {
	configDir := filepath.Dir(configFilePath)
	defaultLogFileName := meta.CLIName + ".log"
	defaultLogPath := filepath.Join(configDir, "logs", defaultLogFileName)

	defaultConfig := map[string]any{
		profileName: map[string]any{
			"output":                    "text",
			"log-file":                  defaultLogPath,
			"konnect":                   map[string]any{},
			common.ColorThemeConfigPath: common.DefaultColorTheme,
		},
	}
	return defaultConfig
}
