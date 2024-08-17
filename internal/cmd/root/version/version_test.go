package version

import (
	"io"
	"log/slog"
	"testing"

	"github.com/kong/kong-cli/internal/build"
	"github.com/kong/kong-cli/internal/config"
	"github.com/kong/kong-cli/internal/iostreams"
	"github.com/kong/kong-cli/test/cmd"
	testConfig "github.com/kong/kong-cli/test/config"
)

func Test_VersionCmd(t *testing.T) {
	all, _, out, _ := iostreams.NewTestIOStreams()

	outputFormat := "text"

	helper := cmd.MockHelper{
		GetOutputFormatMock: func() (string, error) {
			return outputFormat, nil
		},
		GetConfigMock: func() (config.Hook, error) {
			return &testConfig.MockConfigHook{
				GetBoolMock: func(_ string) bool {
					return false
				},
			}, nil
		},
		GetStreamsMock: func() *iostreams.IOStreams {
			return &all
		},
		GetLoggerMock: func() (*slog.Logger, error) {
			return slog.New(slog.NewTextHandler(io.Discard, nil)), nil
		},
		GetBuildInfoMock: func() (*build.Info, error) {
			return &build.Info{
				Version: "dev",
				Commit:  "unknown",
				Date:    "unknown",
			}, nil
		},
	}

	if err := validate(&helper); err != nil {
		t.Errorf("Error validating context: %v", err)
	}

	if err := run(&helper); err != nil {
		t.Errorf("Error running context: %v", err)
	}

	expectedOutput := "dev\n"
	if output := out.String(); output != expectedOutput {
		t.Errorf("Unexpected output: %s", output)
	}
}

//func Test_VersionCmdJsonOutput(t *testing.T) {
//	_, _, stdout, _ := iostreams.NewTestIOStreams()
//
//	//mockConfigHook := &config.MockConfigHook{
//	//	GetStringMock: func(key string) string {
//	//		if key == constant.OutputConfigPath {
//	//			return "json"
//	//		}
//	//		return ""
//	//	},
//	//	GetBoolMock:  func(_ string) bool { return false },
//	//	SaveMock:     nil,
//	//	BindFlagMock: nil,
//	//}
//
//	helper := cmd.MockHelper{}
//
//	if err := validate(&helper); err != nil {
//		t.Errorf("Error validating context: %v", err)
//	}
//
//	if err := run(&helper); err != nil {
//		t.Errorf("Error running context: %v", err)
//	}
//
//	expectedOutput := `{
//		"version": "dev"
//	}`
//
//	var expected, actual map[string]interface{}
//	err := json.Unmarshal([]byte(expectedOutput), &expected)
//	util.CheckError(err) // sanity check of the test json marshal
//
//	json.Unmarshal(stdout.Bytes(), &actual)
//
//	if !reflect.DeepEqual(expected, actual) {
//		t.Errorf("Output does not match expected.\nExpected: %v\nReceived: %v\n", expected, actual)
//	}
//}
//
//func Test_VersionCmdTableOutput(t *testing.T) {
//	//streams := iostreams.NewTestIOStreamsOnly()
//
//	//mockConfigHook := &config.MockConfigHook{
//	//	GetStringMock: func(key string) string {
//	//		if key == constant.OutputConfigPath {
//	//			return "table"
//	//		}
//	//		return ""
//	//	},
//	//	GetBoolMock:  nil,
//	//	SaveMock:     nil,
//	//	BindFlagMock: nil,
//	//}
//
//	helper := cmd.MockHelper{}
//
//	if err := validate(&helper); err == nil {
//		t.Errorf("Expected error, but validate() passed")
//	}
//}
