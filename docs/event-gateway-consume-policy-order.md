# Event Gateway consume policy order

The YAML sequence of consume policies is their desired execution order.
Top-level policies form one chain per virtual cluster. Each parent's children
form a separate sibling chain, including children of `schema_validation`.
A child's position among top-level declarations does not change the parent's
position: only its position relative to siblings with the same parent matters.

For a virtual cluster, declarations are combined in this order:

1. Policies nested in virtual clusters inside `event_gateways`.
2. Policies nested in root-level `event_gateway_virtual_clusters`.
3. Root-level `event_gateway_virtual_cluster_consume_policies`.

Within each group, declaration order is preserved. Files are loaded in `-f`
argument order; directory entries are visited in filename order. Prefer one
sequence per sibling chain to make its intended order easy to review.

`plan` and `diff` observe the authoritative consume policy chain, independently
of the resource list. An ordering update shows the current and desired
`policy_order` sequences. `apply` uses sibling-scoped moves after policy
mutations complete. Otherwise unchanged policies retain their IDs. Sibling
creates execute sequentially; independent chains can execute concurrently.
Saved plans include the ordering intent and dependencies. Newly created or
recreated policy IDs are resolved from the final collection before moving.

In apply mode, declared policies form a prefix of each declared sibling chain.
Undeclared policies remain after that prefix, retaining their relative order.
No ordering operation deletes policies. Sync mode retains its normal deletion
semantics for undeclared policies. Chains without declared policies have no
ordering intent.

Dump exports policies in authoritative execution order, with parent references
that survive reload. Missing or inconsistent chain observations and failed
moves are errors. Replanning after successful reconciliation produces no
ordering changes.

## Other Event Gateway policy chains

The produce, listener, and cluster policy planners also compare individual
policy contents without reconciling sequence order. They remain outside this
consume-policy fix and are tracked in
[#2758](https://github.com/Kong/kongctl/issues/2758). Parent dependency
support in those planners does not establish sibling execution order.
