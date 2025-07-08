package helpers

import "testing"

// This is a mock implementation of the SDKAPI interface
type MockKonnectSDK struct {
	Token                    string
	T                        *testing.T
	CPAPIFactory             func() ControlPlaneAPI
	PortalFactory            func() PortalAPI
	APIFactory               func() APIFullAPI
	APIDocumentFactory       func() APIDocumentAPI
	APIVersionFactory        func() APIVersionAPI
	APIPublicationFactory    func() APIPublicationAPI
	APIImplementationFactory func() APIImplementationAPI
	AppAuthStrategiesFactory func() AppAuthStrategiesAPI
}

// Returns a mock instance of the ControlPlaneAPI
func (m *MockKonnectSDK) GetControlPlaneAPI() ControlPlaneAPI {
	return m.CPAPIFactory()
}

// Returns a mock instance of the PortalAPI
func (m *MockKonnectSDK) GetPortalAPI() PortalAPI {
	if m.PortalFactory != nil {
		return m.PortalFactory()
	}
	return nil
}

// Returns a mock instance of the APIAPI
func (m *MockKonnectSDK) GetAPIAPI() APIFullAPI {
	if m.APIFactory != nil {
		return m.APIFactory()
	}
	return nil
}

// Returns a mock instance of the APIDocumentAPI
func (m *MockKonnectSDK) GetAPIDocumentAPI() APIDocumentAPI {
	if m.APIDocumentFactory != nil {
		return m.APIDocumentFactory()
	}
	return nil
}

// Returns a mock instance of the APIVersionAPI
func (m *MockKonnectSDK) GetAPIVersionAPI() APIVersionAPI {
	if m.APIVersionFactory != nil {
		return m.APIVersionFactory()
	}
	return nil
}

// Returns a mock instance of the APIPublicationAPI
func (m *MockKonnectSDK) GetAPIPublicationAPI() APIPublicationAPI {
	if m.APIPublicationFactory != nil {
		return m.APIPublicationFactory()
	}
	return nil
}

// Returns a mock instance of the APIImplementationAPI
func (m *MockKonnectSDK) GetAPIImplementationAPI() APIImplementationAPI {
	if m.APIImplementationFactory != nil {
		return m.APIImplementationFactory()
	}
	return nil
}

// Returns a mock instance of the AppAuthStrategiesAPI
func (m *MockKonnectSDK) GetAppAuthStrategiesAPI() AppAuthStrategiesAPI {
	if m.AppAuthStrategiesFactory != nil {
		return m.AppAuthStrategiesFactory()
	}
	return nil
}

// This is a mock implementation of the SDKFactory interface
// which can associate a Testing.T instance with any MockKonnectSDK
// instances Built by it
type MockKonnectSDKFactory struct {
	T *testing.T
}

// Returns the mock implementation of the SDKAPI interface
func (m *MockKonnectSDKFactory) Build(token string) (SDKAPI, error) {
	return &MockKonnectSDK{
		Token: token,
		T:     m.T,
	}, nil
}
