# Plan Artifact Workflow Example

This example demonstrates the complete workflow of using plan artifacts for safe, 
reviewable deployments to Kong Konnect.

## Overview

Plan artifacts enable a two-phase deployment process:
1. **Planning Phase**: Generate a plan artifact that captures all changes
2. **Execution Phase**: Apply the plan artifact after review and approval

## Directory Structure

```
plan-artifacts/
├── README.md           # This file
├── config/            # Configuration files
│   ├── api.yaml       # API definitions
│   └── portal.yaml    # Portal configuration
├── plans/             # Generated plan artifacts (git-ignored)
└── scripts/           # Workflow automation scripts
    ├── generate-plan.sh
    └── apply-plan.sh
```

## Workflow Steps

### Step 1: Generate a Plan Artifact

```bash
# Generate a timestamped plan
./scripts/generate-plan.sh

# This creates: plans/plan-2024-01-15-143025.json
```

### Step 2: Review the Plan

View human-readable diff:
```bash
kongctl diff --plan plans/plan-2024-01-15-143025.json
```

Inspect plan details:
```bash
jq '.summary' plans/plan-2024-01-15-143025.json
```

Check specific changes:
```bash
jq '.changes[] | select(.resource_type == "api")' plans/plan-2024-01-15-143025.json
```

### Step 3: Share for Approval

Option 1: Attach to Pull Request
The plan file can be committed or attached as a PR artifact

Option 2: Upload to shared storage
```bash
aws s3 cp plans/plan-2024-01-15-143025.json \
  s3://team-bucket/kong-plans/pending-review/
```

### Step 4: Apply the Plan

After approval, apply the specific plan:
```bash
./scripts/apply-plan.sh plans/plan-2024-01-15-143025.json
```

## Example Configuration

### config/api.yaml
```yaml
apis:
  - ref: example-api
    name: "Example API"
    description: "API managed via plan artifacts"
    version: "v1.0.0"
```

### config/portal.yaml
```yaml
portals:
  - ref: dev-portal
    name: "developer-portal"
    display_name: "Developer Portal"
    authentication_enabled: false
    
api_publications:
  - ref: example-api-pub
    api: example-api
    portal: dev-portal
```

## Scripts

### scripts/generate-plan.sh
```bash
#!/bin/bash
set -e

# Create plans directory if it doesn't exist
mkdir -p plans

# Generate timestamp
TIMESTAMP=$(date +%Y-%m-%d-%H%M%S)
PLAN_FILE="plans/plan-${TIMESTAMP}.json"

echo "Generating plan artifact..."
kongctl plan -f config/ --output-file "$PLAN_FILE"

echo "Plan created: $PLAN_FILE"
echo ""
echo "Review with: kongctl diff --plan $PLAN_FILE"
```

### scripts/apply-plan.sh
```bash
#!/bin/bash
set -e

if [ -z "$1" ]; then
  echo "Usage: $0 <plan-file>"
  exit 1
fi

PLAN_FILE="$1"

if [ ! -f "$PLAN_FILE" ]; then
  echo "Error: Plan file not found: $PLAN_FILE"
  exit 1
fi

echo "Applying plan: $PLAN_FILE"
echo ""

# Show summary
echo "Plan summary:"
jq '.summary' "$PLAN_FILE"
echo ""

# Confirm before applying
read -p "Apply this plan? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  echo "Aborted"
  exit 1
fi

# Apply the plan
kongctl apply --plan "$PLAN_FILE"

# Archive the applied plan
ARCHIVE_DIR="plans/applied"
mkdir -p "$ARCHIVE_DIR"
mv "$PLAN_FILE" "$ARCHIVE_DIR/"

echo "Plan applied and archived to $ARCHIVE_DIR"
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Kong Deployment

on:
  pull_request:
    paths:
      - 'config/**'

jobs:
  plan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Generate Plan
        run: |
          kongctl plan -f config/ --output-file plan.json
          
      - name: Comment PR
        uses: actions/github-script@v6
        with:
          script: |
            const fs = require('fs');
            const plan = JSON.parse(fs.readFileSync('plan.json'));
            
            const body = `## Kong Configuration Plan
            
            **Summary:**
            - Create: ${plan.summary.create}
            - Update: ${plan.summary.update}
            - Delete: ${plan.summary.delete}
            
            Review the plan artifact: [plan.json](${artifactUrl})`;
            
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: body
            });
```

## Best Practices

1. **Always review diffs**: Use `kongctl diff --plan` before applying
2. **Version your plans**: Include timestamps or version numbers
3. **Archive applied plans**: Keep a history of what was deployed
4. **Validate before applying**: Use `--dry-run` to validate plans
5. **Document plan contents**: Include comments about what changed

## Troubleshooting

### Plan is out of date

Regenerate the plan with current state:
```bash
kongctl plan -f config/ --output-file new-plan.json
```

### Viewing plan details

Full plan content:
```bash
jq . plan.json
```

Just the changes:
```bash
jq '.changes[]' plan.json | less
```

### Comparing plans

See what's different between two plans:
```bash
diff <(jq -S . plan1.json) <(jq -S . plan2.json)
```