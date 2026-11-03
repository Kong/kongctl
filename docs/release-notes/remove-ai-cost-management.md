# Breaking bug fix: remove AI Cost Management support

AI Cost Management support was mistakenly exposed in v1.23.0 and has been
removed. This is a breaking bug fix, with no deprecation period.

The following commands are removed:

- `kongctl get ai-cost-management tag-capture`
- `kongctl list ai-cost-management tag-capture`
- `kongctl get konnect ai-cost-management tag-capture`
- `kongctl list konnect ai-cost-management tag-capture`

Remove the `ai_cost_management_tags` collection from declarative manifests
and dump resource selectors. Manifests and explicit dump selectors using it
now fail validation. Saved plans targeting it are rejected before execution;
remove the collection from the source manifest and regenerate those plans.

Normal plan, apply, sync, and dump operations no longer access the tag-capture
API. Existing Konnect tag-capture configuration is left untouched. All other
AI Gateway commands, resources, and runtime header capture remain supported.
