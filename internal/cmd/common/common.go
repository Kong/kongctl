package common

import "fmt"

// Represents an enum of valid values for the format of the output for this CLI execution
type OutputFormat int

type LogLevel int

const (
	JSON OutputFormat = iota
	YAML
	TEXT
)

const (
	TRACE LogLevel = iota
	DEBUG
	INFO
	WARN
	ERROR
)

const (
	// related to the --output flag
	DefaultOutputFormat = "text"
	OutputFlagName      = "output"
	OutputFlagShort     = "o"
	OutputConfigPath    = OutputFlagName

	// related to the --profile flag
	ProfileFlagName  = "profile"
	ProfileFlagShort = "p"

	// related to the --config-file flag
	ConfigFilePathFlagName = "config-file"

	// related to the --log-level flag
	LogLevelFlagName   = "log-level"
	DefaultLogLevel    = "info"
	LogLevelConfigPath = LogLevelFlagName
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
	return [...]string{"trace", "debug", "info", "warn", "error"}[ll]
}

func LogLevelStringToIota(level string) (LogLevel, error) {
	switch level {
	case "trace":
		return TRACE, nil
	case "debug":
		return DEBUG, nil
	case "info":
		return INFO, nil
	case "warn":
		return WARN, nil
	case "error":
		return ERROR, nil
	default:
		return ERROR, fmt.Errorf("invalid log level %q, must be one of %v", level, 
			[]string{"trace", "debug", "info", "warn", "error"})
	}
}
