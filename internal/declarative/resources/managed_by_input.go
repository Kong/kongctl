package resources

// Apply the same reserved-field rule to loading, explain, and scaffold output,
// including SDK unions and nested declarations. Public config/label keys are
// unaffected: only registered resource boundaries are visited.
func applyManagedByInputPolicy(node *ExplainNode, kind ResourceType) {
	if node == nil {
		return
	}
	explainRemoveField(node, "managed_by")
	node.rejectLoadField("managed_by",
		"managed_by is reserved for Kong tools and cannot be set in declarative configuration; "+
			"use labels for user metadata")
	for _, branch := range node.OneOf {
		applyManagedByInputPolicy(branch, kind)
	}
	for _, relation := range explainNestedRelations(kind, registry[kind].explain.typ) {
		if field, ok := node.property(relation.FieldName); ok && field.Node != nil {
			child := field.Node
			if relation.FieldArray {
				child = child.Items
			}
			applyManagedByInputPolicy(child, ResourceType(relation.ChildAlias))
		}
	}
}
