package profile

import (
	"testing"
)

//func buildViper(kvPairs map[string]interface{}) *viper.Viper {
//	v := viper.New()
//	for key, value := range kvPairs {
//		v.Set(key, value)
//	}
//	return v
//}

func TestAddProfile(_ *testing.T) {
	//tests := []struct {
	//	name string
	//	v    *viper.Viper
	//	want error
	//}{
	//	{
	//		name: "",
	//		v:    nil,
	//		want: errorProfileNameEmpty,
	//	},
	//	{
	//		name: "exists",
	//		v:    buildViper(map[string]interface{}{"exists": map[string]interface{}{}}),
	//		want: errorProfileExists,
	//	},
	//	{
	//		name: "valid",
	//		v:    buildViper(map[string]interface{}{}),
	//		want: nil,
	//	},
	//}

	//for _, tt := range tests {
	//	t.Run(tt.name, func(t *testing.T) {
	//		err := CreateProfile(tt.name, tt.v)
	//		if !errors.Is(err, tt.want) {
	//			t.Errorf("CreateProfile() = %v, want %v", err, tt.want)
	//		}
	//	})
	//}
}
