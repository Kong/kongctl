# Organization-chart imports and ordinary configuration

An organization chart comes from the native CSV import operation. The importer
creates user principals, teams and manager relationships. It is distinct from
the ordinary declarative principal/group examples and from the broader planned
arbitrary-principal CSV importer.

The [synthetic CSV example](org-chart.csv) demonstrates the format accepted by
the tested US `.tech` API. Required headers are `external_id`, `email`,
`given_name`, `family_name` and `manager_external_id`. The example's root has an
empty manager cell; other manager values refer to external IDs in the file.
Optional `cost_center` omission preserves existing values; a blank cell removes
the existing metadata key. Other canonical-format rules need the API contract
tracked in Q48. This branch does not yet expose an import command or an import
declaration.

The API accepts multipart `file` and `dry_run` fields:

```http
POST /v2/directories/{directoryId}/org-chart/import
Content-Type: multipart/form-data
```

Use `dry_run=true` to inspect a report without writing the hierarchy. Check its
`errors` array even on HTTP 200. An actual import reconciles the hierarchy;
absent imported users can become inactive and emptied teams can disappear.
It can also adopt an existing API-created user by email. It is not safe to
treat a partial file as an incremental list of users to add.

After importing through the native workflow, inspect the default directory:

```sh
./kongctl get identity principals --directory-name default -o json
./kongctl get identity groups --directory-name default -o json
./kongctl get identity group-memberships \
  --directory-name default --group-id TEAM_UUID -o json
```

Imported users and teams expose `provisioned_by: imported`. Existing users
adopted by import retain their IDs and may retain old `managed_by` values.
Those markers do not mean an imported principal remains writable through
ordinary PUT: the tested API returns 409. KongCTL rejects managed principal
declarations, scoped pruning and adoption for that state. Saved writes repeat
the check; unscoped omission leaves those records alone.

Directory dump exports imported principals as external UUID references,
including users with retained KongCTL markers. Ordinary imported teams and
memberships remain outside automatic configuration pruning. Independently
owned child resources continue to follow their own ownership rules.

Explicit native deactivation changes an imported principal to `konnect_user`
and permits deletion in the tested environment. KongCTL does not silently
perform that release to make a declarative operation succeed. Q49 asks for the
supported transfer/re-import contract, while Q50 records public response-schema
gaps. These questions are in the [API question log][questions].

[questions]: ../../../kong-identity-api-questions.md
