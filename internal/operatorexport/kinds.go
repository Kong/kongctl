package operatorexport

// Kong Operator API groups, versions, and kinds. These are external schema
// identifiers defined by the Kong Operator CRDs.
const (
	apiVersionKonnectV1alpha2 = "konnect.konghq.com/v1alpha2"
	apiVersionConfigV1        = "configuration.konghq.com/v1"
	apiVersionConfigV1alpha1  = "configuration.konghq.com/v1alpha1"
	apiVersionConfigV1beta1   = "configuration.konghq.com/v1beta1"

	configurationGroup = "configuration.konghq.com"
	konnectGroup       = "konnect.konghq.com"

	KindReferenceGrant     = "KongReferenceGrant"
	KindControlPlane       = "KonnectGatewayControlPlane"
	KindService            = "KongService"
	KindRoute              = "KongRoute"
	KindUpstream           = "KongUpstream"
	KindTarget             = "KongTarget"
	KindConsumer           = "KongConsumer"
	KindConsumerGroup      = "KongConsumerGroup"
	KindPlugin             = "KongPlugin"
	KindPluginBinding      = "KongPluginBinding"
	KindCredentialACL      = "KongCredentialACL"       //nolint:gosec // CRD kind, not a credential
	KindCredentialAPIKey   = "KongCredentialAPIKey"    //nolint:gosec // CRD kind, not a credential
	KindCredentialBasic    = "KongCredentialBasicAuth" //nolint:gosec // CRD kind, not a credential
	KindCredentialHMAC     = "KongCredentialHMAC"      //nolint:gosec // CRD kind, not a credential
	KindCredentialJWT      = "KongCredentialJWT"       //nolint:gosec // CRD kind, not a credential
	KindCertificate        = "KongCertificate"
	KindCACertificate      = "KongCACertificate"
	KindSNI                = "KongSNI"
	KindVault              = "KongVault"
	KindKey                = "KongKey"
	KindKeySet             = "KongKeySet"
	KindDataPlaneClientCrt = "KongDataPlaneClientCertificate"
)

// Manifest field names shared by every resource.
const (
	fieldName = "name"
	fieldSpec = "spec"
)

// Adoption modes. Match and override are written to spec.adopt.mode; auto
// picks one of them per kind.
const (
	AdoptModeMatch    = "match"
	AdoptModeOverride = "override"
	AdoptModeAuto     = "auto"
)

// Control plane cluster types that cannot own adopted entities.
const (
	clusterTypeGroup = "CLUSTER_TYPE_CONTROL_PLANE_GROUP"
	clusterTypeKIC   = "CLUSTER_TYPE_K8S_INGRESS_CONTROLLER"
)

// reservedTagPrefixes are tag prefixes the Kong Operator CRDs reject in
// spec.tags because the operator writes them itself.
var reservedTagPrefixes = []string{
	"k8s-group:",
	"k8s-version:",
	"k8s-kind:",
	"k8s-namespace:",
	"k8s-name:",
	"k8s-uid:",
	"k8s-generation:",
	"managed-by:",
}

const (
	k8sUIDTagPrefix = "k8s-uid:"
	// maxCRDTags is the CRD limit on spec.tags entries.
	maxCRDTags = 20
	// maxPreservedUserTags is how many user tags survive once the operator
	// appends its own eight tags under the same 20 tag cap.
	maxPreservedUserTags = 12
)
