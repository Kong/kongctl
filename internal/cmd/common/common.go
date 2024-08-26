package common

import "fmt"

type OutputFormat int
type LogLevel int

const (
	// related to the --output flag
	DefaultOutputFormat              = "text"
	OutputFlagName                   = "output"
	OutputFlagShort                  = "o"
	OutputConfigPath                 = OutputFlagName
	JSON                OutputFormat = iota
	YAML
	TEXT

	// related to the --profile flag
	ProfileFlagName  = "profile"
	ProfileFlagShort = "p"

	// related to the --config-file flag
	ConfigFilePathFlagName = "config-file"

	// related to the --log-level flag
	LogLevelFlagName            = "log-level"
	DefaultLogLevel             = "info"
	LogLevelConfigPath          = LogLevelFlagName
	DEBUG              LogLevel = iota
	INFO
	WARN
	ERROR
)

func (of OutputFormat) String() string {
	return [...]string{"json", "yaml", "text"}[of]
}

func OutputFormatStringToIota(format string) (OutputFormat, error) {
	switch format {
	case "json":
		return JSON, nil
	case "yaml":
		return YAML, nil
	case "text":
		return TEXT, nil
	default:
		return TEXT, fmt.Errorf("invalid output format %q, must be one of %v", format, []string{"json", "yaml", "text"})
	}
}

func (ll LogLevel) String() string {
	return [...]string{"debug", "info", "warn", "error"}[ll]
}

func LogLevelStringToIota(level string) (LogLevel, error) {
	switch level {
	case "debug":
		return DEBUG, nil
	case "info":
		return INFO, nil
	case "warn":
		return WARN, nil
	case "error":
		return ERROR, nil
	default:
		return ERROR, fmt.Errorf("invalid log level %q, must be one of %v", level, []string{"debug", "info", "warn", "error"})
	}
}
