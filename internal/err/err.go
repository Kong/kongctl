package err

import "encoding/json"

// ConfigurationError represents errors that are a result of bad flags, combinations of
// flags, configuration settings, environment values, or other command usage issues.
type ConfigurationError struct {
	Err error
}

// ExecutionError represents errors that occur after a command has been validated and an
// unsuccessful result occurs.  Network errors, server side errors, invalid credentials or responses
// are examples of RunttimeError types.
type ExecutionError struct {
	// friendly error message to display to the user
	Msg string
	// Err is the error that occurred during execution
	Err error
	// Optional attributes that can be used to provide additional context to the error
	Attrs []interface{}
}

func (e *ConfigurationError) Error() string {
	return e.Err.Error()
}

func (e *ExecutionError) Error() string {
	return e.Err.Error()
}

// Will try and json unmarshal an error string into a slice of interfaces
// that match the slog algorithm for varadic parameters (alternating key value pairs)
func TryConvertErrorToAttrs(err error) []interface{} {
	var result map[string]interface{}
	umError := json.Unmarshal([]byte(err.Error()), &result)
	if umError != nil {
		return nil
	}
	attrs := make([]interface{}, 0, len(result)*2)
	for k, v := range result {
		attrs = append(attrs, k, v)
	}
	return attrs
}

type ErrorsBucket struct {
	Msg    string
	Errors []error
}

func (e *ErrorsBucket) Error() string {
	s := e.Msg
	for _, err := range e.Errors {
		s += "\n\t" + err.Error()
	}
	return s
}
