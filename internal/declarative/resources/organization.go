package resources

// OrganizationResource represents an organization grouping in declarative configuration.
// It's a grouping concept that can contain nested resources like teams.
type OrganizationResource struct {
	// Teams nested under this organization
	Teams []OrganizationTeamResource `yaml:"teams,omitempty" json:"teams,omitempty"`
}
