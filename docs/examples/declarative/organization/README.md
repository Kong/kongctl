# Organization Examples

These examples demonstrate how to manage Konnect organization resources with
`kongctl` declarative configuration.

## Important: system account selectors

System accounts are selector-only for now. The Konnect API does not currently
support labels on system accounts, so `kongctl` cannot manage them by namespace.
Any system account referenced in these examples must already exist in the
organization and must be selected by `name` or `id`.

## Files

- `teams.yaml` - defines organization teams with namespaces.
- `team-roles.yaml` - defines organization teams and assigns roles to API and
  portal resources using `!ref` in `entity_id`. The example shows both nested
  `organization.teams[].roles` declarations and root-level
  `organization_team_roles` declarations.
- `user-assignments.yaml` - assigns existing organization users to teams and
  direct roles. Users have a local `ref` and are selected by exactly one of
  `email` or `id`; team memberships and roles also have local refs. Assignments
  can be nested or declared at the root with
  `organization_user_team_memberships` and `organization_user_roles`. Root
  assignments identify their selector with `user`. kongctl does not create or
  delete users.
- `system-account-assignments.yaml` - assigns existing organization system
  accounts to teams and direct roles. System accounts have a local `ref` and
  are selected by exactly one of `name` or `id`; team memberships and roles
  also have local refs. Assignments can be nested or declared at the root with
  `organization_system_account_team_memberships` and
  `organization_system_account_roles`. Root assignments identify their
  selector with `system_account`. kongctl does not create or delete system
  accounts.

## Usage

Preview the team role change set:

```bash
kongctl diff -f team-roles.yaml
```

Apply the team role configuration:

```bash
kongctl apply -f team-roles.yaml
```

Apply user team memberships and direct user roles:

```bash
kongctl apply -f user-assignments.yaml
```

Apply system account team memberships and direct system account roles:

```bash
kongctl apply -f system-account-assignments.yaml
```

Run in sync mode to remove omitted managed role assignments in the namespace:

```bash
kongctl sync -f team-roles.yaml
```

Dump organization teams with child role assignments:

```bash
kongctl dump declarative \
  --resources=organization.teams,apis \
  --include-child-resources
```
