package products

// Empty type to represent the _type_ Product. Genesis is to support a key in a Context
type ProductKey struct{}

// Product is a global instance of the ProductKey type
var Product = ProductKey{}

// Will represent a specific Product (konnect, gateway, mesh, etc)
type ProductValue string

func (p ProductValue) String() string {
	return string(p)
}
