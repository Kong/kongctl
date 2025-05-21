package helpers

import (
	"log/slog"

	kkInternal "github.com/Kong/sdk-konnect-go-internal"
	kkSDK "github.com/Kong/sdk-konnect-go" // kk = Kong Konnect
	"github.com/kong/kongctl/internal/config"
)

// Provides an interface for the Konnect Go SDK
// "github.com/Kong/sdk-konnect-go" SDK struct
// allowing for easier testing and mocking
type SDKAPI interface {
	GetControlPlaneAPI() ControlPlaneAPI
	GetPortalAPI() PortalAPI
	GetAPIAPI() APIAPI
}

// This is the real implementation of the SDKAPI
// which wraps the actual SDK implmentation
type KonnectSDK struct {
	SDK            *kkSDK.SDK
	InternalSDK    *kkInternal.SDK
	internalPortal *InternalPortalAPI
	internalAPI    *InternalAPIAPI
}

// Returns the real implementation of the GetControlPlaneAPI
// from the Konnect SDK
func (k *KonnectSDK) GetControlPlaneAPI() ControlPlaneAPI {
	return k.SDK.ControlPlanes
}

// Returns the implementation of the PortalAPI interface
// for accessing the Developer Portal APIs using the internal SDK
func (k *KonnectSDK) GetPortalAPI() PortalAPI {
	if k.internalPortal == nil && k.InternalSDK != nil {
		k.internalPortal = &InternalPortalAPI{
			SDK: k.InternalSDK,
		}
	}
	return k.internalPortal
}

// Returns the implementation of the APIAPI interface
// for accessing the API APIs using the internal SDK
func (k *KonnectSDK) GetAPIAPI() APIAPI {
	if k.internalAPI == nil && k.InternalSDK != nil {
		k.internalAPI = &InternalAPIAPI{
			SDK: k.InternalSDK,
		}
	}
	return k.internalAPI
}

// A function that can build an SDKAPI with a given configuration
type SDKAPIFactory func(cfg config.Hook, logger *slog.Logger) (SDKAPI, error)

type Key struct{}

// A Key used to store the SDKFactory in a Context
var SDKAPIFactoryKey = Key{}
