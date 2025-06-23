# KongCtl Stage 2 - Plan Generation with Label Management

## Goal
Build the planner that compares current vs desired state and generates plans with CREATE/UPDATE operations.

## Deliverables
- Konnect API integration for fetching current portal state
- Label management system (KONGCTL/managed, KONGCTL/config-hash)
- Plan generation for CREATE and UPDATE operations
- Plan serialization to JSON format
- Basic `diff` command implementation

## Implementation Details

### Konnect Client Wrapper
```go
// internal/declarative/state/client.go
type KonnectClient struct {
    sdk *konnectsdk.SDK
}

func (c *KonnectClient) ListPortals(ctx context.Context) ([]components.Portal, error) {
    // Wrapper around SDK with error handling
    // Filter to only KONGCTL/managed resources
}

func (c *KonnectClient) GetPortalByName(ctx context.Context, name string) (*components.Portal, error) {
    // Find portal by name among managed resources
}
```

### Label Management
```go
// internal/declarative/resources/labels.go
const (
    LabelManaged     = "KONGCTL/managed"
    LabelConfigHash  = "KONGCTL/config-hash"
    LabelLastUpdated = "KONGCTL/last-updated"
    LabelProtected   = "KONGCTL/protected"
)

func AddManagedLabels(labels map[string]string, configHash string) map[string]string {
    if labels == nil {
        labels = make(map[string]string)
    }
    labels[LabelManaged] = "true"
    labels[LabelConfigHash] = configHash
    labels[LabelLastUpdated] = time.Now().UTC().Format(time.RFC3339)
    return labels
}

func IsManagedResource(labels map[string]string) bool {
    return labels[LabelManaged] == "true"
}
```

### Plan Structure
```go
// internal/declarative/planner/plan.go
type Plan struct {
    Metadata PlanMetadata    `json:"metadata"`
    Changes  []PlannedChange `json:"changes"`
    Summary  PlanSummary     `json:"summary"`
}

type PlannedChange struct {
    ID           string      `json:"id"`
    ResourceType string      `json:"resource_type"`
    ResourceName string      `json:"resource_name"`
    Action       ActionType  `json:"action"` // CREATE, UPDATE
    CurrentState interface{} `json:"current_state,omitempty"`
    DesiredState interface{} `json:"desired_state"`
    FieldChanges []FieldChange `json:"field_changes,omitempty"`
}

type ActionType string
const (
    ActionCreate ActionType = "CREATE"
    ActionUpdate ActionType = "UPDATE"
)
```

### Suggested Planner Implementation
```go
// internal/declarative/planner/planner.go
type Planner struct {
    client *state.KonnectClient
}

func (p *Planner) GeneratePlan(ctx context.Context, resources *ResourceSet) (*Plan, error) {
    plan := &Plan{
        Metadata: generateMetadata(),
        Changes:  []PlannedChange{},
    }
    
    // 1. Fetch current state (only KONGCTL/managed resources)
    currentPortals, err := p.client.ListPortals(ctx)
    
    // 2. Compare each desired portal
    for _, desired := range resources.Portals {
        configHash := calculateConfigHash(desired)
        current := findPortalByName(currentPortals, desired.Name)
        
        if current == nil {
            // CREATE
            change := PlannedChange{
                Action:       ActionCreate,
                ResourceType: "portal",
                ResourceName: desired.Name,
                DesiredState: desired,
            }
            plan.Changes = append(plan.Changes, change)
        } else if current.Labels[LabelConfigHash] != configHash {
            // UPDATE
            change := PlannedChange{
                Action:       ActionUpdate,
                ResourceType: "portal",
                ResourceName: desired.Name,
                CurrentState: current,
                DesiredState: desired,
                FieldChanges: calculateFieldChanges(current, desired),
            }
            plan.Changes = append(plan.Changes, change)
        }
    }
    
    plan.Summary = calculateSummary(plan.Changes)
    return plan, nil
}
```

### Suggested Diff Command Implementation
```go
// internal/cmd/root/verbs/diff/diff.go
func Execute(cmd *cobra.Command, args []string) error {
    // 1. Load declarative configuration
    resources, err := loadResources(configDir)
    
    // 2. Generate plan
    planner := planner.New(konnectClient)
    plan, err := planner.GeneratePlan(ctx, resources)
    
    // 3. Display diff
    if outputFormat == "json" {
        return json.NewEncoder(os.Stdout).Encode(plan)
    }
    
    return displayHumanReadableDiff(plan)
}
```

## Tests Required
- Plan generation with various state combinations (no current state, matching state, drift)
- Label management functions
- Config hash calculation consistency
- Diff output formatting
- Mock Konnect API responses

## Proof of Success
```bash
# Generate and view a plan
$ kongctl plan -o plan.json
Plan generated: 1 change(s) detected

$ kongctl diff --plan plan.json
Portal developer-portal:
  + to be created
  labels:
    + KONGCTL/managed: "true"
    + KONGCTL/config-hash: "abc123..."
    
# View plan as JSON
$ kongctl diff --plan plan.json --output json
{
  "metadata": {
    "generated_at": "2024-01-20T10:30:00Z",
    "version": "1.0"
  },
  "changes": [{
    "id": "change-001",
    "resource_type": "portal",
    "resource_name": "developer-portal",
    "action": "CREATE",
    "desired_state": {...}
  }],
  "summary": {
    "total_changes": 1,
    "by_action": {"CREATE": 1}
  }
}
```

## Dependencies
- Stage 1 completion (configuration format and loading)
- Kong SDK for API operations
- Crypto library for hash generation

## Notes
- Only manage resources with KONGCTL/managed label
- Config hash enables fast drift detection
- Plan format must support future extension (DELETE actions, dependencies)
- Consider rate limiting when fetching current state