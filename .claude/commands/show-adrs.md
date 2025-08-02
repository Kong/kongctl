# show-adrs

Display architecture decision records (ADRs) for the current feature.

## Steps

1. Identify the current feature:
   - Read `planning/index.md` to find current active stage
   - Note the feature folder name

2. Read the ADRs file:
   - Navigate to the feature folder
   - Open `execution-plan-adrs.md` if it exists
   - If no ADRs file exists, report that no ADRs are documented

3. Parse and summarize ADRs:
   - Look for sections starting with "ADR-" (e.g., "ADR-001-001")
   - For each ADR, extract:
     - ADR number and title
     - Decision summary
     - Context/reasoning (brief)
     - Status (accepted, rejected, etc.)

4. Display organized summary:
   - List all ADRs with their titles
   - Provide brief summaries
   - Group by topic if applicable
   - Show status of each decision

5. Offer to show details:
   - Ask if user wants full details of any specific ADR
   - Provide guidance on how ADRs relate to implementation

## Example Output

```
Architecture Decision Records
============================

Feature: Declarative Configuration - Stage 1
ADR File: planning/001-dec-cfg-cfg-format-basic-cli/execution-plan-adrs.md

📋 Decision Summary:

ADR-001-001: Type-specific ResourceSet structure
Status: ✅ Accepted
Summary: Use separate ResourceSet types for each resource category instead of generic map
Reasoning: Provides type safety and better IDE support

ADR-001-002: SDK type embedding for configuration
Status: ✅ Accepted  
Summary: Embed SDK types directly in configuration structures
Reasoning: Avoids duplication and ensures consistency with API

ADR-001-003: Separate ref field for cross-resource references
Status: ✅ Accepted
Summary: Use dedicated 'ref' field instead of inline references
Reasoning: Clear separation between configuration and references

ADR-001-008: Per-resource reference mappings
Status: ✅ Accepted
Summary: Maintain reference maps per resource type instead of global map
Reasoning: Prevents naming conflicts and improves organization

Total: 4 decisions documented

Use: "Show me details for ADR-001-XXX" to see full context for any decision.
These ADRs guide the implementation - refer to them when questions arise about the approach.
```