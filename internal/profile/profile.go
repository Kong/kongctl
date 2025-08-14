package profile

import (
	"errors"
	"strings"

	"github.com/spf13/viper"
)

const (
	DefaultProfile = "default"
)

var (
	errorProfileExists    = errors.New("profile already exists")
	errorProfileNameEmpty = errors.New("invalid profile name (empty)")
)

type Manager interface {
	GetProfiles() []string
	GetProfile(name string) (map[string]any, error)
	CreateProfile(name string) error
	DeleteProfile(name string) error
}

type profileManager struct {
	config *viper.Viper
}

// Empty type to represent the _type_ Manager. Genesis is to support a key in a Context
type Key struct{}

// Global instance of the ProfileManagerKey type
var ProfileManagerKey = Key{}

func (v *profileManager) GetProfiles() []string {
	allKeys := v.config.AllKeys()
	keyMap := make(map[string]bool)

	for _, key := range allKeys {
		topLevelKey := strings.Split(key, ".")[0]
		keyMap[topLevelKey] = true
	}

	uniqueTopLevelKeys := make([]string, 0, len(keyMap))
	for key := range keyMap {
		uniqueTopLevelKeys = append(uniqueTopLevelKeys, key)
	}

	return uniqueTopLevelKeys
}

func (v *profileManager) CreateProfile(profileName string) error {
	if profileName == "" {
		return errorProfileNameEmpty
	}

	if v.config.IsSet(profileName) {
		return errorProfileExists
	}

	v.config.Set(profileName, map[string]any{})

	return nil
}

func (v *profileManager) DeleteProfile(_ string) error {
	//if !v.IsSet(name) {
	//	return errorProfileDoesNotExist
	//}

	// v.Set(name, nil)
	// v.config.Set
	// v.creds.Delete(profileName)

	return nil
}

func (v *profileManager) GetProfile(name string) (map[string]any, error) {
	return v.config.GetStringMap(name), nil
}

func NewManager(config *viper.Viper) Manager {
	return &profileManager{
		config: config,
	}
}
