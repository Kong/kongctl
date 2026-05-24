# Organization Examples

These examples demonstrate how to manage Konnect organization resources with
`kongctl` declarative configuration.

## Files

- `teams.yaml` - defines organization teams with namespaces.
- `team-roles.yaml` - defines organization teams and assigns roles to API and
  portal resources using `!ref` in `entity_id`. The example shows both nested
  `organization.teams[].roles` declarations and root-level
  `organization_team_roles` declarations.
- `user-assignments.yaml` - assigns existing organization users to teams and
  direct roles. Users have a local `ref` and are selected by exactly one of
  `email` or `id`; team memberships and roles also have local refs. kongctl
  does not create or delete users.
- `system-account-assignments.yaml` - assigns existing organization system
  accounts to teams and direct roles. System accounts have a local `ref` and
  are selected by exactly one of `name` or `id`; team memberships and roles
  also have local refs. kongctl does not create or delete system accounts.

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
