package resources

// AnalyticsResource represents the analytics grouping in declarative configuration.
// It's a grouping concept that can contain nested analytics resources.
type AnalyticsResource struct {
	// Dashboards nested under analytics
	Dashboards []DashboardResource `yaml:"dashboards,omitempty" json:"dashboards,omitempty"`
}
