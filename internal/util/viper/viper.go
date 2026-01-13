package viper

import (
	"strings"

	"github.com/kong/kongctl/internal/util"
	v "github.com/spf13/viper"
)

// InitializeDefaultViper initializes a viper instance with default values and a path to a file
// If the file does not exist, it will be created with the default values
func InitializeDefaultViper(defaultValues map[string]any, path string) (*v.Viper, error) {
	var err error

	err = util.InitDir(path, 0o755)
	if err != nil {
		return nil, err
	}

	rv := NewViper(path)

	if len(rv.AllSettings()) == 0 {
		// the 'loaded' viper is empty, so we assume it's uninitialized and
		// set the default and the write back to the file
		err = rv.MergeConfigMap(defaultValues)
		if err != nil {
			return nil, err
		}
		// And write it back to the file
		err = rv.WriteConfig()
		if err != nil {
			return nil, err
		}
	}

	return rv, err
}

func NewViperE(path string) (*v.Viper, error) {
	rv := v.New()
	rv.SetConfigFile(path)
	rv.AutomaticEnv()
	rv.SetEnvPrefix("kongctl")
	rv.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	err := rv.ReadInConfig()
	if err != nil {
		return nil, err
	}
	return rv, nil
}

func NewViper(path string) *v.Viper {
	rv := v.New()
	rv.SetConfigFile(path)
	rv.AutomaticEnv()
	rv.SetEnvPrefix("kongctl")
	rv.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	_ = rv.ReadInConfig()
	return rv
}

func PersistViper(v *v.Viper) error {
	// v.Write
	return v.WriteConfig()
}
