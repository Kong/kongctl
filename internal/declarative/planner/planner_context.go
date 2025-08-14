package planner

// Config contains configuration data needed during planning operations.
// This replaces the anti-pattern of passing data through context.WithValue.
type Config struct {
	// Namespace specifies the target namespace for planning operations
	Namespace string
}

// NewConfig creates a new planner config with the specified namespace.
func NewConfig(namespace string) *Config {
	return &Config{
		Namespace: namespace,
	}
}