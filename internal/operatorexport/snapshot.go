// Package operatorexport converts existing Konnect gateway entities into Kong
// Operator custom resources that adopt those entities.
//
// The package is pure: callers fetch a Snapshot from Konnect and Build turns it
// into manifests. Adopted entities are written back in full by the operator on
// its first sync, so every spec reproduces the Konnect entity field for field
// and anything a CRD cannot hold is reported as a Warning rather than dropped.
package operatorexport

import (
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// ControlPlane identifies the Konnect gateway control plane being exported.
type ControlPlane struct {
	ID          string
	Name        string
	ClusterType string
}

// Snapshot holds every entity read from one Konnect gateway control plane.
type Snapshot struct {
	ControlPlane ControlPlane

	Services       []kkComps.Service
	Routes         []kkComps.Route
	Upstreams      []kkComps.Upstream
	Targets        []kkComps.Target
	Consumers      []kkComps.Consumer
	ConsumerGroups []kkComps.ConsumerGroup
	// ConsumerGroupMembers maps a consumer group ID to the IDs of its consumers.
	ConsumerGroupMembers map[string][]string
	Plugins              []kkComps.Plugin

	ACLs       []kkComps.ACL
	BasicAuths []kkComps.BasicAuth
	HMACAuths  []kkComps.HMACAuth
	JWTs       []kkComps.Jwt
	KeyAuths   []kkComps.KeyAuth

	Certificates          []kkComps.Certificate
	CACertificates        []kkComps.CACertificate
	SNIs                  []kkComps.Sni
	Vaults                []kkComps.Vault
	Keys                  []kkComps.Key
	KeySets               []kkComps.KeySet
	DataPlaneCertificates []kkComps.DataPlaneClientCertificate
}
