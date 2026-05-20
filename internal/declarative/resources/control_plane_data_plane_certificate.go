package resources

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypeControlPlaneDataPlaneCertificate,
		func(rs *ResourceSet) *[]ControlPlaneDataPlaneCertificateResource {
			return &rs.ControlPlaneDataPlaneCertificates
		},
		AutoExplain[ControlPlaneDataPlaneCertificateResource](
			WithExplainFieldHint("cert", ExplainFieldHint{
				FileSample:   "./certs/data-plane.pem",
				PreferredTag: "!file",
				Notes: []string{
					"Certificate contents identify this resource within its control plane.",
					"Use !file or !env to avoid inlining PEM data in configuration.",
				},
			}),
			WithExplainRecommendedFields("ref", "control_plane", "cert"),
		),
	)
}

// ControlPlaneDataPlaneCertificateResource represents a data plane certificate
// pinned to an API Gateway control plane.
type ControlPlaneDataPlaneCertificateResource struct {
	Ref          string `yaml:"ref"                     json:"ref"`
	ControlPlane string `yaml:"control_plane,omitempty" json:"control_plane,omitempty"`
	Cert         string `yaml:"cert"                    json:"cert"`

	konnectID string `yaml:"-" json:"-"`
}

func (c ControlPlaneDataPlaneCertificateResource) GetType() ResourceType {
	return ResourceTypeControlPlaneDataPlaneCertificate
}

func (c ControlPlaneDataPlaneCertificateResource) GetRef() string {
	return c.Ref
}

func (c ControlPlaneDataPlaneCertificateResource) GetMoniker() string {
	return ShortControlPlaneDataPlaneCertificateIdentity(c.Cert)
}

func (c ControlPlaneDataPlaneCertificateResource) GetDependencies() []ResourceRef {
	if c.ControlPlane == "" {
		return nil
	}
	return []ResourceRef{{Kind: ResourceTypeControlPlane, Ref: c.ControlPlane}}
}

func (c ControlPlaneDataPlaneCertificateResource) GetReferenceFieldMappings() map[string]string {
	if c.ControlPlane == "" {
		return nil
	}
	return map[string]string{"control_plane": string(ResourceTypeControlPlane)}
}

func (c ControlPlaneDataPlaneCertificateResource) GetKonnectID() string {
	return c.konnectID
}

func (c ControlPlaneDataPlaneCertificateResource) Validate() error {
	if err := ValidateRef(c.Ref); err != nil {
		return fmt.Errorf("invalid control_plane_data_plane_certificate ref: %w", err)
	}
	if c.ControlPlane == "" {
		return fmt.Errorf("control_plane is required")
	}
	if c.Cert == "" {
		return fmt.Errorf("cert is required")
	}
	return nil
}

func (c *ControlPlaneDataPlaneCertificateResource) SetDefaults() {
	// No defaults: the remote API has no name or display field for this resource.
}

func (c ControlPlaneDataPlaneCertificateResource) GetKonnectMonikerFilter() string {
	return ""
}

func (c *ControlPlaneDataPlaneCertificateResource) TryMatchKonnectResource(konnectResource any) bool {
	cert, id := dataPlaneCertificateFields(konnectResource)
	if id == "" || cert == "" || cert != c.Cert {
		return false
	}
	c.konnectID = id
	return true
}

func (c ControlPlaneDataPlaneCertificateResource) GetParentRef() *ResourceRef {
	if c.ControlPlane == "" {
		return nil
	}
	return &ResourceRef{Kind: ResourceTypeControlPlane, Ref: c.ControlPlane}
}

// ControlPlaneDataPlaneCertificateIdentity returns the stable identity for a
// certificate. The exact certificate contents are the remote identity; hashing
// keeps map keys and plan labels compact without changing matching semantics.
func ControlPlaneDataPlaneCertificateIdentity(cert string) string {
	sum := sha256.Sum256([]byte(cert))
	return hex.EncodeToString(sum[:])
}

func ShortControlPlaneDataPlaneCertificateIdentity(cert string) string {
	identity := ControlPlaneDataPlaneCertificateIdentity(cert)
	if len(identity) <= 12 {
		return identity
	}
	return identity[:12]
}

func dataPlaneCertificateFields(konnectResource any) (cert string, id string) {
	switch typed := konnectResource.(type) {
	case kkComps.DataPlaneClientCertificate:
		return stringPtrValue(typed.Cert), stringPtrValue(typed.ID)
	case *kkComps.DataPlaneClientCertificate:
		if typed == nil {
			return "", ""
		}
		return stringPtrValue(typed.Cert), stringPtrValue(typed.ID)
	default:
		return "", ""
	}
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
