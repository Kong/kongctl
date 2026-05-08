# Organization Examples

These examples demonstrate how to manage Konnect organization resources with
`kongctl` declarative configuration.

## Files

- `teams.yaml` - defines organization teams with namespaces.
- `team-roles.yaml` - defines organization teams and assigns roles to an API.
  The example shows both nested `organization.teams[].roles` declarations and
  root-level `organization_team_roles` declarations.
- `user-assignments.yaml` - assigns existing organization users to teams and
  direct roles. Users are selected by exactly one of `email` or `id`; kongctl
  does not create or delete users.

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
