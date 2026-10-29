package resources

// SchemaFieldMeteringBilling is the declaration grouping for Metering & Billing.
const SchemaFieldMeteringBilling = "metering-billing"

// MeteringBillingResource groups the independently managed billing collections.
type MeteringBillingResource struct {
	Meters          []MeterResource          `yaml:"meters,omitempty" json:"meters,omitempty"`
	Features        []FeatureResource        `yaml:"features,omitempty" json:"features,omitempty"`
	TaxCodes        []TaxCodeResource        `yaml:"tax_codes,omitempty" json:"tax_codes,omitempty"`
	Currencies      []CurrencyResource       `yaml:"currencies,omitempty" json:"currencies,omitempty"`
	BillingApps     []BillingAppResource     `yaml:"apps,omitempty" json:"apps,omitempty"`
	BillingProfiles []BillingProfileResource `yaml:"profiles,omitempty" json:"profiles,omitempty"`
	BillingPlans    []BillingPlanResource    `yaml:"plans,omitempty" json:"plans,omitempty"`
}
