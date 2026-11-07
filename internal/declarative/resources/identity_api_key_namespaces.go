package resources

// External principals confer no lifecycle authority. Their generated keys use
// an immutable namespace in addition to the existing creation/ref markers.
// Managed principals retain their original inherited-authority convention.
func withPrincipalAPIKeyNamespaces() ResourceRegistrationOption {
	participant := func(k *IdentityPrincipalAPIKeyResource) NamespaceParticipant {
		return NamespaceParticipant{Ref: k.Ref, Label: string(ResourceTypeIdentityPrincipalAPIKey), Meta: &k.Kongctl}
	}
	return func(ops *resourceOps) error {
		if err := WithNamespace(216, participant)(ops); err != nil {
			return err
		}
		if err := WithNestedNamespaceGroups(func(rs *ResourceSet) [][]IdentityPrincipalAPIKeyResource {
			var groups [][]IdentityPrincipalAPIKeyResource
			visitIdentityPrincipals(rs, func(p *IdentityPrincipalResource) { groups = append(groups, p.APIKeys) })
			return groups
		}, participant)(ops); err != nil {
			return err
		}
		ops.matchesNamespace = func(rs *ResourceSet, value Resource, namespace string) bool {
			key := value.(*IdentityPrincipalAPIKeyResource)
			parent := identityPrincipalByRef(rs, key.Principal)
			if parent == nil {
				return false
			}
			if parent.IsExternal() {
				return GetNamespace(key.Kongctl) == namespace
			}
			return GetNamespace(parent.Kongctl) == namespace
		}
		// Namespace-only ownership deliberately has no protection accessor: the
		// five-entry API limit leaves no room for mutable protection metadata.
		ops.captureOwnedChildScope = capturePrincipalAPIKeyScopes
		return nil
	}
}

func capturePrincipalAPIKeyScopes(rs *ResourceSet, defaultNamespace string) {
	if rs.Identity == nil {
		return
	}
	scope := rs.EnsureSyncScope()
	visitIdentityPrincipals(rs, func(p *IdentityPrincipalResource) {
		if !p.IsExternal() {
			return
		}
		var namespaces []string
		for _, key := range p.APIKeys {
			namespaces = append(namespaces, GetNamespace(key.Kongctl))
		}
		CaptureOwnedChildCollection(scope, ResourceTypeIdentityPrincipalAPIKey, p.Ref, defaultNamespace,
			scope.ChildInScope(ResourceTypeIdentityPrincipal, p.Ref, ResourceTypeIdentityPrincipalAPIKey), namespaces)
	})
	for _, key := range rs.Identity.PrincipalAPIKeys {
		parent := identityPrincipalByRef(rs, key.Principal)
		if parent != nil && parent.IsExternal() {
			scope.AddChildNamespace(ResourceTypeIdentityPrincipalAPIKey, key.Principal, GetNamespace(key.Kongctl))
		}
	}
}
