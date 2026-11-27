package operatorexport

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// Options control how manifests are generated.
type Options struct {
	// Namespace is the Kubernetes namespace for namespaced resources.
	Namespace string
	// AuthRef names the KonnectAPIAuthConfiguration the control plane uses.
	AuthRef string
	// AdoptMode selects spec.adopt.mode: match, override, or auto, which
	// picks the mode per kind (see modeFor).
	AdoptMode string
	// IncludeSecrets writes secret material (credential secrets, private
	// keys) inline. When false, entities that need it are skipped.
	IncludeSecrets bool
}

// Manifest is one Kubernetes object ready to be serialized.
type Manifest struct {
	Kind   string
	Name   string
	Object map[string]any
}

// Warning describes an entity that was skipped or exported with a caveat.
type Warning struct {
	Kind    string
	Name    string
	ID      string
	Message string
}

func (w Warning) String() string {
	subject := w.Kind
	if w.Name != "" {
		subject += " " + w.Name
	}
	if w.ID != "" {
		subject += " (" + w.ID + ")"
	}
	return subject + ": " + w.Message
}

// Result is the output of Build.
type Result struct {
	Manifests []Manifest
	Warnings  []Warning
}

// Validate checks the options before a build.
func (o Options) Validate() error {
	if strings.TrimSpace(o.Namespace) == "" {
		return errors.New("namespace is required")
	}
	if sanitizeName(o.Namespace) != o.Namespace {
		return fmt.Errorf("namespace %q is not a valid Kubernetes namespace name", o.Namespace)
	}
	if strings.TrimSpace(o.AuthRef) == "" {
		return errors.New("auth reference is required")
	}
	switch o.AdoptMode {
	case AdoptModeAuto, AdoptModeMatch, AdoptModeOverride:
	default:
		return fmt.Errorf("adopt mode must be %q, %q, or %q, got %q",
			AdoptModeAuto, AdoptModeMatch, AdoptModeOverride, o.AdoptMode)
	}
	return nil
}

// overrideKinds are the kinds auto mode adopts with override. Against Kong
// Operator 2.4, match adoption fails for them with NotMatch even when nothing
// differs, while override was verified to leave the Konnect entity unchanged:
//   - KongPluginBinding: the operator compares Konnect's plugin config,
//     including unset (null) fields, with the KongPlugin config, which
//     Kubernetes stores without nulls.
//   - KongUpstream: the operator compares healthchecks by pointer.
//
// Every other kind uses match, because override adoption of a KongService
// clears its client and CA certificate references in Konnect.
var overrideKinds = []string{KindPluginBinding, KindUpstream}

// modeFor returns the spec.adopt.mode written for a kind.
func (b *builder) modeFor(kind string) string {
	if b.opts.AdoptMode != AdoptModeAuto {
		return b.opts.AdoptMode
	}
	if slices.Contains(overrideKinds, kind) {
		return AdoptModeOverride
	}
	return AdoptModeMatch
}

type builder struct {
	opts   Options
	snap   Snapshot
	names  *nameAllocator
	result Result

	// cpName is the KonnectGatewayControlPlane object name.
	cpName string
	// exported records, per kind, the IDs that produced a manifest.
	exported map[string]map[string]bool
}

// Build converts a snapshot into Kong Operator manifests.
func Build(snap Snapshot, opts Options) (Result, error) {
	if err := opts.Validate(); err != nil {
		return Result{}, err
	}
	switch snap.ControlPlane.ClusterType {
	case clusterTypeGroup:
		return Result{}, fmt.Errorf("control plane %q is a control plane group; "+
			"Kong Operator cannot attach entities to a group, export its member control planes instead",
			snap.ControlPlane.Name)
	case clusterTypeKIC:
		return Result{}, fmt.Errorf("control plane %q is managed by Kong Ingress Controller; "+
			"its entities are already owned by Kubernetes", snap.ControlPlane.Name)
	}

	b := &builder{
		opts:     opts,
		snap:     snap,
		names:    newNameAllocator(),
		exported: map[string]map[string]bool{},
	}
	b.cpName = sanitizeName(snap.ControlPlane.Name)
	if b.cpName == "" {
		b.cpName = "control-plane-" + shortID(snap.ControlPlane.ID)
	}
	b.proposeNames()

	steps := []func() error{
		b.addControlPlane,
		b.addVaults,
		b.addCertificates,
		b.addCACertificates,
		b.addSNIs,
		b.addKeySets,
		b.addKeys,
		b.addDataPlaneCertificates,
		b.addUpstreams,
		b.addTargets,
		b.addServices,
		b.addRoutes,
		b.addConsumerGroups,
		b.addConsumers,
		b.addCredentials,
		b.addPlugins,
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return Result{}, err
		}
	}
	return b.result, nil
}

// proposeNames registers every entity up front so references resolve to the
// same name whichever order the kinds are emitted in.
func (b *builder) proposeNames() {
	s := b.snap
	for _, v := range s.Vaults {
		// KongVault is cluster scoped, so its name must be unique across
		// every control plane exported into the cluster.
		b.names.propose(KindVault, strValue(v.ID), b.cpName+"-"+v.Prefix, b.cpName+"-vault")
	}
	for _, c := range s.Certificates {
		b.names.propose(KindCertificate, strValue(c.ID), "", "certificate")
	}
	for _, c := range s.CACertificates {
		b.names.propose(KindCACertificate, strValue(c.ID), "", "ca-certificate")
	}
	for _, sni := range s.SNIs {
		b.names.propose(KindSNI, strValue(sni.ID), sni.Name, "sni")
	}
	for _, ks := range s.KeySets {
		b.names.propose(KindKeySet, strValue(ks.ID), strValue(ks.Name), "key-set")
	}
	for _, k := range s.Keys {
		preferred := strValue(k.Name)
		if preferred == "" {
			preferred = k.Kid
		}
		b.names.propose(KindKey, strValue(k.ID), preferred, "key")
	}
	for _, c := range s.DataPlaneCertificates {
		b.names.propose(KindDataPlaneClientCrt, strValue(c.ID), "", "dataplane-certificate")
	}
	for _, u := range s.Upstreams {
		b.names.propose(KindUpstream, strValue(u.ID), u.Name, "upstream")
	}
	for _, t := range s.Targets {
		b.names.propose(KindTarget, strValue(t.ID), strValue(t.Target), "target")
	}
	for _, svc := range s.Services {
		b.names.propose(KindService, strValue(svc.ID), strValue(svc.Name), "service")
	}
	for _, r := range s.Routes {
		id, name := routeIDAndName(r)
		b.names.propose(KindRoute, id, name, "route")
	}
	for _, g := range s.ConsumerGroups {
		b.names.propose(KindConsumerGroup, strValue(g.ID), g.Name, "consumer-group")
	}
	for _, c := range s.Consumers {
		preferred := strValue(c.Username)
		if preferred == "" {
			preferred = strValue(c.CustomID)
		}
		b.names.propose(KindConsumer, strValue(c.ID), preferred, "consumer")
	}
	for _, p := range s.Plugins {
		preferred := strValue(p.InstanceName)
		if preferred == "" {
			preferred = p.Name
		}
		// KongPlugin and KongPluginBinding share a name per plugin.
		b.names.propose(KindPlugin, strValue(p.ID), preferred, "plugin")
	}
}

func routeIDAndName(r kkComps.Route) (string, string) {
	switch {
	case r.RouteJSON != nil:
		return strValue(r.RouteJSON.ID), strValue(r.RouteJSON.Name)
	case r.RouteExpression != nil:
		return strValue(r.RouteExpression.ID), strValue(r.RouteExpression.Name)
	default:
		return "", ""
	}
}

func (b *builder) warn(kind, name, id, format string, args ...any) {
	b.result.Warnings = append(b.result.Warnings, Warning{
		Kind: kind, Name: name, ID: id, Message: fmt.Sprintf(format, args...),
	})
}

func (b *builder) markExported(kind, id string) {
	if b.exported[kind] == nil {
		b.exported[kind] = map[string]bool{}
	}
	b.exported[kind][id] = true
}

// ref returns the object name for a referenced entity. When the referenced
// entity was not exported it still returns the name the entity would have, so
// the reference is preserved rather than cleared, and warns that the
// referenced resource must be supplied separately.
func (b *builder) ref(fromKind, fromName, fromID, toKind, toID string) string {
	name := b.names.name(toKind, toID)
	if name == "" {
		b.warn(fromKind, fromName, fromID, "references %s %s, which does not exist in the control plane", toKind, toID)
		return sanitizeName(toKind) + "-" + shortID(toID)
	}
	if !b.exported[toKind][toID] {
		b.warn(fromKind, fromName, fromID,
			"references %s %s, which was not exported; it must exist before this resource can be adopted",
			toKind, name)
	}
	return name
}

func (b *builder) metadata(name string, namespaced bool) map[string]any {
	meta := map[string]any{fieldName: name}
	if namespaced {
		meta["namespace"] = b.opts.Namespace
	}
	return meta
}

func (b *builder) adopt(id string, mode string) map[string]any {
	return map[string]any{
		"from": "konnect",
		"mode": mode,
		"konnect": map[string]any{
			"id": id,
		},
	}
}

func (b *builder) controlPlaneRef(withNamespace bool) map[string]any {
	ref := map[string]any{fieldName: b.cpName}
	if withNamespace {
		ref["namespace"] = b.opts.Namespace
	}
	return map[string]any{
		"type":                 "konnectNamespacedRef",
		"konnectNamespacedRef": ref,
	}
}

func namespacedRef(name string) map[string]any {
	return map[string]any{fieldName: name}
}

// applyTags writes cleaned tags into spec and reports whether the entity may
// be exported; entities already owned by another Kubernetes resource may not.
func (b *builder) applyTags(kind, name, id string, tags []string, spec map[string]any) bool {
	kept, removed, owned := cleanTags(tags)
	if owned {
		b.warn(kind, name, id, "skipped: already managed by a Kubernetes resource (has a k8s-uid tag)")
		return false
	}
	if len(removed) > 0 {
		b.warn(kind, name, id, "removed reserved tags the operator manages itself: %s", strings.Join(removed, ", "))
	}
	if len(kept) > maxCRDTags {
		b.warn(kind, name, id, "skipped: has %d tags, more than the %d the CRD allows", len(kept), maxCRDTags)
		return false
	}
	if len(kept) > maxPreservedUserTags {
		b.warn(kind, name, id,
			"has %d tags; the operator adds its own tags under a %d tag limit, so some may be dropped on sync",
			len(kept), maxCRDTags)
	}
	if len(kept) > 0 {
		spec["tags"] = kept
	}
	return true
}

func (b *builder) warnUnrepresented(kind, name, id string, fields []string) {
	if len(fields) == 0 {
		return
	}
	b.warn(kind, name, id,
		"fields %s have no equivalent in the %s CRD; the operator will not preserve them on sync",
		strings.Join(fields, ", "), kind)
}

func (b *builder) emit(kind, apiVersion, name, id string, namespaced bool, body map[string]any) {
	obj := map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   b.metadata(name, namespaced),
	}
	maps.Copy(obj, body)
	b.result.Manifests = append(b.result.Manifests, Manifest{Kind: kind, Name: name, Object: obj})
	b.markExported(kind, id)
}

// emitSpec emits a resource whose only body field is its spec.
func (b *builder) emitSpec(kind, apiVersion, name, id string, namespaced bool, spec map[string]any) {
	b.emit(kind, apiVersion, name, id, namespaced, map[string]any{fieldSpec: spec})
}

func (b *builder) addControlPlane() error {
	b.emit(KindControlPlane, apiVersionKonnectV1alpha2, b.cpName, b.snap.ControlPlane.ID, true, map[string]any{
		fieldSpec: map[string]any{
			"source": "Mirror",
			"mirror": map[string]any{
				"konnect": map[string]any{"id": b.snap.ControlPlane.ID},
			},
			"konnect": map[string]any{
				"authRef": map[string]any{fieldName: b.opts.AuthRef},
			},
		},
	})
	return nil
}

// specFor builds a spec from an SDK entity. It returns a nil spec when the
// entity must be skipped.
func (b *builder) specFor(
	kind, name, id string, entity any, tags []string, allowed, handled []string,
) (map[string]any, error) {
	fields, err := toFieldMap(entity)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", kind, id, err)
	}
	spec, unrepresented := pickFields(fields, allowed, handled)
	if !b.applyTags(kind, name, id, tags, spec) {
		return nil, nil
	}
	b.warnUnrepresented(kind, name, id, unrepresented)
	return spec, nil
}

func (b *builder) addVaults() error {
	exported := false
	for _, v := range b.snap.Vaults {
		id := strValue(v.ID)
		name := b.names.name(KindVault, id)
		if !b.opts.IncludeSecrets && vaultConfigHasSecrets(v.Config) {
			b.warn(
				KindVault,
				name,
				id,
				"skipped: config contains credentials; rerun with --include-secrets to export it",
			)
			continue
		}
		spec, err := b.specFor(KindVault, name, id, v, v.Tags,
			[]string{"prefix", "description", "config"}, []string{fieldName})
		if err != nil {
			return err
		}
		if spec == nil {
			continue
		}
		spec["backend"] = v.Name
		// KongVault is cluster scoped, so the control plane reference must
		// name its namespace.
		spec["controlPlaneRef"] = b.controlPlaneRef(true)
		spec["adopt"] = b.adopt(id, b.modeFor(KindVault))
		b.emitSpec(KindVault, apiVersionConfigV1alpha1, name, id, false, spec)
		exported = true
	}
	if exported {
		b.addVaultReferenceGrant()
	}
	return nil
}

// addVaultReferenceGrant lets cluster-scoped KongVaults reference the
// namespaced control plane. The operator rejects that cross-namespace
// reference unless a grant in the control plane's namespace allows it, and a
// cluster-scoped source is matched by an empty namespace.
func (b *builder) addVaultReferenceGrant() {
	name := "kongvault-to-" + b.cpName
	b.emitSpec(KindReferenceGrant, apiVersionConfigV1alpha1, name, "", true, map[string]any{
		"from": []any{map[string]any{
			"group":     configurationGroup,
			"kind":      KindVault,
			"namespace": "",
		}},
		"to": []any{map[string]any{
			"group":   konnectGroup,
			"kind":    KindControlPlane,
			fieldName: b.cpName,
		}},
	})
}

// vaultSecretKeyMarkers identify vault config keys that hold credentials.
var vaultSecretKeyMarkers = []string{"token", "secret", "password", "private_key", "api_key"}

func vaultConfigHasSecrets(config map[string]any) bool {
	for key, value := range config {
		lower := strings.ToLower(key)
		for _, marker := range vaultSecretKeyMarkers {
			if strings.Contains(lower, marker) && !isEmptyValue(value) {
				return true
			}
		}
	}
	return false
}

func (b *builder) addCertificates() error {
	for _, c := range b.snap.Certificates {
		id := strValue(c.ID)
		name := b.names.name(KindCertificate, id)
		if !b.opts.IncludeSecrets {
			b.warn(KindCertificate, name, id,
				"skipped: contains a private key; rerun with --include-secrets to export it")
			continue
		}
		spec, err := b.specFor(KindCertificate, name, id, c, c.Tags,
			[]string{"cert", "key", "cert_alt", "key_alt"}, nil)
		if err != nil {
			return err
		}
		if spec == nil {
			continue
		}
		spec["type"] = "inline"
		spec["controlPlaneRef"] = b.controlPlaneRef(false)
		spec["adopt"] = b.adopt(id, b.modeFor(KindCertificate))
		b.emitSpec(KindCertificate, apiVersionConfigV1alpha1, name, id, true, spec)
	}
	return nil
}

func (b *builder) addCACertificates() error {
	for _, c := range b.snap.CACertificates {
		id := strValue(c.ID)
		name := b.names.name(KindCACertificate, id)
		spec, err := b.specFor(KindCACertificate, name, id, c, c.Tags,
			[]string{"cert"}, []string{"cert_digest"})
		if err != nil {
			return err
		}
		if spec == nil {
			continue
		}
		spec["type"] = "inline"
		spec["controlPlaneRef"] = b.controlPlaneRef(false)
		spec["adopt"] = b.adopt(id, b.modeFor(KindCACertificate))
		b.emitSpec(KindCACertificate, apiVersionConfigV1alpha1, name, id, true, spec)
	}
	return nil
}

func (b *builder) addSNIs() error {
	for _, sni := range b.snap.SNIs {
		id := strValue(sni.ID)
		name := b.names.name(KindSNI, id)
		spec, err := b.specFor(KindSNI, name, id, sni, sni.Tags,
			[]string{fieldName}, []string{"certificate"})
		if err != nil {
			return err
		}
		if spec == nil {
			continue
		}
		certID := strValue(sni.Certificate.ID)
		spec["certificateRef"] = namespacedRef(b.ref(KindSNI, name, id, KindCertificate, certID))
		spec["adopt"] = b.adopt(id, b.modeFor(KindSNI))
		b.emitSpec(KindSNI, apiVersionConfigV1alpha1, name, id, true, spec)
	}
	return nil
}

func (b *builder) addKeySets() error {
	for _, ks := range b.snap.KeySets {
		id := strValue(ks.ID)
		name := b.names.name(KindKeySet, id)
		if strValue(ks.Name) == "" {
			b.warn(KindKeySet, name, id, "skipped: the KongKeySet CRD requires a name and this key set has none")
			continue
		}
		spec, err := b.specFor(KindKeySet, name, id, ks, ks.Tags, []string{fieldName}, nil)
		if err != nil {
			return err
		}
		if spec == nil {
			continue
		}
		spec["controlPlaneRef"] = b.controlPlaneRef(false)
		spec["adopt"] = b.adopt(id, b.modeFor(KindKeySet))
		b.emitSpec(KindKeySet, apiVersionConfigV1alpha1, name, id, true, spec)
	}
	return nil
}

func (b *builder) addKeys() error {
	for _, k := range b.snap.Keys {
		id := strValue(k.ID)
		name := b.names.name(KindKey, id)
		if !b.opts.IncludeSecrets && keyHasPrivateMaterial(k) {
			b.warn(
				KindKey,
				name,
				id,
				"skipped: contains private key material; rerun with --include-secrets to export it",
			)
			continue
		}
		spec, err := b.specFor(KindKey, name, id, k, k.Tags,
			[]string{"kid", fieldName, "jwk", "pem"}, []string{"set", "x5t"})
		if err != nil {
			return err
		}
		if spec == nil {
			continue
		}
		if setID := keySetID(k); setID != "" {
			spec["keySetRef"] = map[string]any{
				"type":          "namespacedRef",
				"namespacedRef": namespacedRef(b.ref(KindKey, name, id, KindKeySet, setID)),
			}
		}
		spec["controlPlaneRef"] = b.controlPlaneRef(false)
		spec["adopt"] = b.adopt(id, b.modeFor(KindKey))
		b.emitSpec(KindKey, apiVersionConfigV1alpha1, name, id, true, spec)
	}
	return nil
}

func keySetID(k kkComps.Key) string {
	if k.Set == nil {
		return ""
	}
	return strValue(k.Set.ID)
}

// keyHasPrivateMaterial reports whether a key carries a PEM private key or a
// JWK with private parameters.
func keyHasPrivateMaterial(k kkComps.Key) bool {
	if k.Pem != nil && strValue(k.Pem.PrivateKey) != "" {
		return true
	}
	jwk := strValue(k.Jwk)
	if jwk == "" {
		return false
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(jwk), &parsed); err != nil {
		// An unparsable JWK cannot be inspected; treat it as sensitive.
		return true
	}
	for _, param := range []string{"d", "p", "q", "dp", "dq", "qi", "k"} {
		if _, ok := parsed[param]; ok {
			return true
		}
	}
	return false
}

func (b *builder) addDataPlaneCertificates() error {
	for _, c := range b.snap.DataPlaneCertificates {
		id := strValue(c.ID)
		name := b.names.name(KindDataPlaneClientCrt, id)
		if b.opts.AdoptMode == AdoptModeOverride {
			b.warn(KindDataPlaneClientCrt, name, id, "exported in match mode; the CRD does not allow override")
		}
		spec := map[string]any{
			"cert":            strValue(c.Cert),
			"controlPlaneRef": b.controlPlaneRef(false),
			"adopt":           b.adopt(id, AdoptModeMatch),
		}
		b.emitSpec(KindDataPlaneClientCrt, apiVersionConfigV1alpha1, name, id, true, spec)
	}
	return nil
}

var upstreamFields = []string{
	"algorithm", "hash_fallback", "hash_fallback_header", "hash_fallback_query_arg",
	"hash_fallback_uri_capture", "hash_on", "hash_on_cookie", "hash_on_cookie_path", "hash_on_header",
	"hash_on_query_arg", "hash_on_uri_capture", "healthchecks", "host_header", fieldName, "slots",
	"sticky_sessions_cookie", "sticky_sessions_cookie_path", "use_srv_name",
}

func (b *builder) addUpstreams() error {
	for _, u := range b.snap.Upstreams {
		id := strValue(u.ID)
		name := b.names.name(KindUpstream, id)
		spec, err := b.specFor(KindUpstream, name, id, u, u.Tags, upstreamFields, []string{"client_certificate"})
		if err != nil {
			return err
		}
		if spec == nil {
			continue
		}
		if u.ClientCertificate != nil && strValue(u.ClientCertificate.ID) != "" {
			spec["clientCertificateRef"] = namespacedRef(
				b.ref(KindUpstream, name, id, KindCertificate, strValue(u.ClientCertificate.ID)))
		}
		if b.modeFor(KindUpstream) == AdoptModeMatch {
			b.warn(KindUpstream, name, id,
				"match mode may report a mismatch because of how the operator compares healthchecks; "+
					"use override if adoption fails with NotMatch")
		}
		spec["controlPlaneRef"] = b.controlPlaneRef(false)
		spec["adopt"] = b.adopt(id, b.modeFor(KindUpstream))
		b.emitSpec(KindUpstream, apiVersionConfigV1alpha1, name, id, true, spec)
	}
	return nil
}

func (b *builder) addTargets() error {
	for _, t := range b.snap.Targets {
		id := strValue(t.ID)
		name := b.names.name(KindTarget, id)
		spec, err := b.specFor(KindTarget, name, id, t, t.Tags,
			[]string{"target", "weight"}, []string{"upstream"})
		if err != nil {
			return err
		}
		if spec == nil {
			continue
		}
		upstreamID := ""
		if t.Upstream != nil {
			upstreamID = strValue(t.Upstream.ID)
		}
		spec["upstreamRef"] = namespacedRef(b.ref(KindTarget, name, id, KindUpstream, upstreamID))
		spec["adopt"] = b.adopt(id, b.modeFor(KindTarget))
		b.emitSpec(KindTarget, apiVersionConfigV1alpha1, name, id, true, spec)
	}
	return nil
}

var serviceFields = []string{
	"connect_timeout", "enabled", "host", fieldName, "path", "port", "protocol", "read_timeout",
	"retries", "tls_verify", "tls_verify_depth", "write_timeout",
}

func (b *builder) addServices() error {
	for _, svc := range b.snap.Services {
		id := strValue(svc.ID)
		name := b.names.name(KindService, id)
		spec, err := b.specFor(KindService, name, id, svc, svc.Tags, serviceFields,
			[]string{"client_certificate", "ca_certificates"})
		if err != nil {
			return err
		}
		if spec == nil {
			continue
		}
		if svc.ClientCertificate != nil && strValue(svc.ClientCertificate.ID) != "" {
			spec["clientCertificateRef"] = namespacedRef(
				b.ref(KindService, name, id, KindCertificate, strValue(svc.ClientCertificate.ID)))
		}
		if len(svc.CaCertificates) > 0 {
			refs := make([]any, 0, len(svc.CaCertificates))
			for _, caID := range svc.CaCertificates {
				refs = append(refs, namespacedRef(b.ref(KindService, name, id, KindCACertificate, caID)))
			}
			spec["caCertificateRefs"] = refs
		}
		if b.modeFor(KindService) == AdoptModeOverride &&
			(spec["clientCertificateRef"] != nil || len(svc.CaCertificates) > 0) {
			b.warn(KindService, name, id,
				"override adoption may clear the client and CA certificate references in Konnect; "+
					"adopt services with --adopt-mode auto or match")
		}
		spec["controlPlaneRef"] = b.controlPlaneRef(false)
		spec["adopt"] = b.adopt(id, b.modeFor(KindService))
		b.emitSpec(KindService, apiVersionConfigV1alpha1, name, id, true, spec)
	}
	return nil
}

var routeFields = []string{
	"destinations", "headers", "hosts", "https_redirect_status_code", "methods", fieldName,
	"path_handling", "paths", "preserve_host", "protocols", "regex_priority", "request_buffering",
	"response_buffering", "snis", "sources", "strip_path",
}

func (b *builder) addRoutes() error {
	for _, r := range b.snap.Routes {
		id, _ := routeIDAndName(r)
		name := b.names.name(KindRoute, id)
		if r.RouteJSON == nil {
			b.warn(KindRoute, name, id, "skipped: expression routes cannot be adopted by the operator")
			continue
		}
		route := r.RouteJSON
		spec, err := b.specFor(KindRoute, name, id, route, route.Tags, routeFields, []string{"service"})
		if err != nil {
			return err
		}
		if spec == nil {
			continue
		}
		if route.Service != nil && strValue(route.Service.ID) != "" {
			spec["serviceRef"] = map[string]any{
				"type":          "namespacedRef",
				"namespacedRef": namespacedRef(b.ref(KindRoute, name, id, KindService, strValue(route.Service.ID))),
			}
		} else {
			spec["controlPlaneRef"] = b.controlPlaneRef(false)
		}
		spec["adopt"] = b.adopt(id, b.modeFor(KindRoute))
		b.emitSpec(KindRoute, apiVersionConfigV1alpha1, name, id, true, spec)
	}
	return nil
}

func (b *builder) addConsumerGroups() error {
	for _, g := range b.snap.ConsumerGroups {
		id := strValue(g.ID)
		name := b.names.name(KindConsumerGroup, id)
		spec, err := b.specFor(KindConsumerGroup, name, id, g, g.Tags, []string{fieldName}, nil)
		if err != nil {
			return err
		}
		if spec == nil {
			continue
		}
		spec["controlPlaneRef"] = b.controlPlaneRef(false)
		spec["adopt"] = b.adopt(id, b.modeFor(KindConsumerGroup))
		b.emitSpec(KindConsumerGroup, apiVersionConfigV1beta1, name, id, true, spec)
	}
	return nil
}

func (b *builder) addConsumers() error {
	groupsByConsumer := map[string][]string{}
	for groupID, consumerIDs := range b.snap.ConsumerGroupMembers {
		for _, consumerID := range consumerIDs {
			groupsByConsumer[consumerID] = append(groupsByConsumer[consumerID], groupID)
		}
	}

	for _, c := range b.snap.Consumers {
		id := strValue(c.ID)
		name := b.names.name(KindConsumer, id)
		spec, err := b.specFor(KindConsumer, name, id, c, c.Tags, nil, []string{"username", "custom_id"})
		if err != nil {
			return err
		}
		if spec == nil {
			continue
		}
		spec["controlPlaneRef"] = b.controlPlaneRef(false)
		spec["adopt"] = b.adopt(id, b.modeFor(KindConsumer))

		body := map[string]any{fieldSpec: spec}
		if username := strValue(c.Username); username != "" {
			body["username"] = username
		}
		if customID := strValue(c.CustomID); customID != "" {
			body["custom_id"] = customID
		}
		// The operator reconciles membership to exactly this list, so it
		// must name every group the consumer belongs to.
		groupIDs := groupsByConsumer[id]
		slices.Sort(groupIDs)
		if len(groupIDs) > 0 {
			groups := make([]any, 0, len(groupIDs))
			for _, groupID := range groupIDs {
				groups = append(groups, b.ref(KindConsumer, name, id, KindConsumerGroup, groupID))
			}
			body["consumerGroups"] = groups
		}
		b.emit(KindConsumer, apiVersionConfigV1, name, id, true, body)
	}
	return nil
}

// credential is the shared shape of a consumer credential being exported.
type credential struct {
	kind       string
	id         string
	consumerID string
	tags       []string
	entity     any
	allowed    []string
	secret     bool
	label      string
}

func (b *builder) addCredentials() error {
	s := b.snap
	for _, ba := range s.BasicAuths {
		id := strValue(ba.ID)
		b.warn(KindCredentialBasic, ba.Username, id,
			"skipped: Konnect only returns the password hash, so the operator would overwrite the password on sync")
	}

	var creds []credential
	for _, acl := range s.ACLs {
		creds = append(creds, credential{
			kind: KindCredentialACL, id: strValue(acl.ID), consumerID: consumerIDOf(acl.Consumer),
			tags: acl.Tags, entity: acl, allowed: []string{"group"}, label: "acl",
		})
	}
	for _, ka := range s.KeyAuths {
		creds = append(creds, credential{
			kind: KindCredentialAPIKey, id: strValue(ka.ID), consumerID: consumerIDOf(ka.Consumer),
			tags: ka.Tags, entity: ka, allowed: []string{"key"}, secret: true, label: "key-auth",
		})
	}
	for _, h := range s.HMACAuths {
		creds = append(creds, credential{
			kind: KindCredentialHMAC, id: strValue(h.ID), consumerID: consumerIDOf(h.Consumer),
			tags: h.Tags, entity: h, allowed: []string{"username", "secret"}, secret: true, label: "hmac-auth",
		})
	}
	for _, j := range s.JWTs {
		creds = append(creds, credential{
			kind: KindCredentialJWT, id: strValue(j.ID), consumerID: consumerIDOf(j.Consumer),
			tags: j.Tags, entity: j, allowed: []string{"algorithm", "key", "rsa_public_key", "secret"},
			secret: true, label: "jwt",
		})
	}

	for _, cred := range creds {
		consumerName := b.names.name(KindConsumer, cred.consumerID)
		name := sanitizeName(consumerName + "-" + cred.label + "-" + shortID(cred.id))
		if cred.secret && !b.opts.IncludeSecrets {
			b.warn(cred.kind, name, cred.id, "skipped: contains a secret; rerun with --include-secrets to export it")
			continue
		}
		spec, err := b.specFor(cred.kind, name, cred.id, cred.entity, cred.tags, cred.allowed, []string{"consumer"})
		if err != nil {
			return err
		}
		if spec == nil {
			continue
		}
		spec["consumerRef"] = namespacedRef(b.ref(cred.kind, name, cred.id, KindConsumer, cred.consumerID))
		spec["adopt"] = b.adopt(cred.id, b.modeFor(cred.kind))
		b.emitSpec(cred.kind, apiVersionConfigV1alpha1, name, cred.id, true, spec)
	}
	return nil
}

// consumerIDOf extracts the consumer ID from any of the SDK's per-credential
// consumer reference types, which all share the same JSON shape.
func consumerIDOf(consumer any) string {
	fields, err := toFieldMap(consumer)
	if err != nil {
		return ""
	}
	id, _ := fields["id"].(string)
	return id
}

func (b *builder) addPlugins() error {
	for _, p := range b.snap.Plugins {
		id := strValue(p.ID)
		name := b.names.name(KindPlugin, id)
		if len(p.Partials) > 0 {
			b.warn(KindPlugin, name, id, "skipped: plugins linked to partials cannot be represented by KongPlugin")
			continue
		}
		if !b.pluginTargetsExportable(p, name, id) {
			continue
		}
		if unsupported := unsupportedPluginProtocols(p.Protocols); len(unsupported) > 0 {
			b.warn(
				KindPlugin,
				name,
				id,
				"skipped: runs on protocols %s, which KongPlugin cannot represent; adopting it would narrow its protocols",
				strings.Join(unsupported, ", "),
			)
			continue
		}

		plugin := map[string]any{"plugin": p.Name}
		if !b.applyTags(KindPlugin, name, id, p.Tags, plugin) {
			continue
		}
		fields, err := toFieldMap(p)
		if err != nil {
			return fmt.Errorf("%s %s: %w", KindPlugin, id, err)
		}
		// Keep the config as Konnect returns it. Kubernetes drops the
		// explicit nulls on admission, which leaves set values unchanged.
		if config, ok := fields["config"]; ok && config != nil {
			plugin["config"] = config
			if b.modeFor(KindPluginBinding) == AdoptModeMatch && configHasNull(config) {
				b.warn(KindPluginBinding, name, id,
					"match mode will fail with NotMatch: the operator compares Konnect's config, including unset "+
						"(null) fields, with the KongPlugin config, which Kubernetes stores without nulls; "+
						"use --adopt-mode override, which writes back the same values")
			}
		}
		if p.Enabled != nil && !*p.Enabled {
			plugin["disabled"] = true
		}
		if protocols, ok := fields["protocols"]; ok && !isEmptyValue(protocols) {
			plugin["protocols"] = protocols
		}
		if condition := strValue(p.Condition); condition != "" {
			plugin["condition"] = condition
		}
		if instance := strValue(p.InstanceName); instance != "" {
			plugin["instance_name"] = instance
		}
		b.emit(KindPlugin, apiVersionConfigV1, name, id, true, plugin)

		spec := map[string]any{
			"pluginRef":       map[string]any{fieldName: name, "kind": KindPlugin},
			"controlPlaneRef": b.controlPlaneRef(false),
			"adopt":           b.adopt(id, b.modeFor(KindPluginBinding)),
		}
		targets := b.pluginTargets(p, name, id)
		if len(targets) == 0 {
			spec["scope"] = "GlobalInControlPlane"
		} else {
			spec["scope"] = "OnlyTargets"
			spec["targets"] = targets
		}
		b.emitSpec(KindPluginBinding, apiVersionConfigV1alpha1, name, id, true, spec)
	}
	return nil
}

// pluginTargetsExportable reports whether every entity a plugin is attached to
// can be referenced. A binding whose target is never created would never
// program, so such plugins are skipped instead.
func (b *builder) pluginTargetsExportable(p kkComps.Plugin, name, id string) bool {
	if p.Route != nil && strValue(p.Route.ID) != "" {
		routeID := strValue(p.Route.ID)
		if !b.exported[KindRoute][routeID] {
			b.warn(KindPlugin, name, id, "skipped: attached to route %s, which was not exported", routeID)
			return false
		}
	}
	return true
}

func (b *builder) pluginTargets(p kkComps.Plugin, name, id string) map[string]any {
	targets := map[string]any{}
	withGroupKind := func(target, kind string) map[string]any {
		return map[string]any{fieldName: target, "kind": kind, "group": configurationGroup}
	}
	if p.Service != nil && strValue(p.Service.ID) != "" {
		targets["serviceRef"] = withGroupKind(
			b.ref(KindPluginBinding, name, id, KindService, strValue(p.Service.ID)), KindService)
	}
	if p.Route != nil && strValue(p.Route.ID) != "" {
		targets["routeRef"] = withGroupKind(
			b.ref(KindPluginBinding, name, id, KindRoute, strValue(p.Route.ID)), KindRoute)
	}
	if p.Consumer != nil && strValue(p.Consumer.ID) != "" {
		targets["consumerRef"] = namespacedRef(
			b.ref(KindPluginBinding, name, id, KindConsumer, strValue(p.Consumer.ID)))
	}
	if p.ConsumerGroup != nil && strValue(p.ConsumerGroup.ID) != "" {
		targets["consumerGroupRef"] = namespacedRef(
			b.ref(KindPluginBinding, name, id, KindConsumerGroup, strValue(p.ConsumerGroup.ID)))
	}
	return targets
}

// kongPluginProtocols are the protocols the KongPlugin CRD accepts.
var kongPluginProtocols = []string{"http", "https", "grpc", "grpcs", "tcp", "tls", "udp"}

func unsupportedPluginProtocols(protocols []kkComps.Protocols) []string {
	var unsupported []string
	for _, protocol := range protocols {
		if !slices.Contains(kongPluginProtocols, string(protocol)) {
			unsupported = append(unsupported, string(protocol))
		}
	}
	return unsupported
}

// configHasNull reports whether a plugin config holds an explicit null at any
// depth.
func configHasNull(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case map[string]any:
		for _, item := range v {
			if configHasNull(item) {
				return true
			}
		}
	case []any:
		if slices.ContainsFunc(v, configHasNull) {
			return true
		}
	}
	return false
}
