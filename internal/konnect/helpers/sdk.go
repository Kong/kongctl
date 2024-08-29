package helpers

import (
	kkSDK "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	"github.com/kong/kongctl/internal/konnect/auth"
)

// Provides an interface for the Konnect Go SDK
// "github.com/Kong/sdk-konnect-go" SDK struct
// allowing for easier testing and mocking
type SDKAPI interface {
	GetControlPlaneAPI() ControlPlaneAPI
}

// This is the real implementation of the SDKAPI
// which wraps the actual SDK implmentation
type KonnectSDK struct {
	*kkSDK.SDK
}

// Returns the real implementation of the GetControlPlaneAPI
// from the Konnect SDK
func (k *KonnectSDK) GetControlPlaneAPI() ControlPlaneAPI {
	return k.ControlPlanes
}

// A function that can build an SDKAPI with a given
// authorization token
type SDKFactory func(token string) (SDKAPI, error)

type Key struct{}

// A Key used to store the SDKFactory in a Context
var SDKFactoryKey = Key{}

// THis is the real implementation of the SDKFactory,
// It creates an Authenticated SDK instance from the adjacent auth package
func KonnectSDKFactory(token string) (SDKAPI, error) {
	sdk, err := auth.GetAuthenticatedClient(token)
	if err != nil {
		return nil, err
	}

	return &KonnectSDK{
		sdk,
	}, nil
}
