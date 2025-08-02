# Implementation Guide for Claude Code

**Streamlined guide for implementing kongctl features using the planning document structure.**

## 🚀 Getting Started

### 1. Check Current Status
Read `planning/index.md` → "Current Active Stage" section shows current development and links to implementation guide.

### 2. Find Your Next Task
Look for first step with Status: "Not Started" and resolved dependencies.

### 3. Implementation Workflow
1. **Update status** → "In Progress"
2. **Read step details** → Complete implementation guide provided
3. **Write code** → Follow examples exactly
4. **Write tests** → Test requirements specified in each step
5. **Commit** → Use commit message template provided
6. **Update status** → "Completed"
7. **Move to next step**

## 📋 Document Organization

### Folder Structure
```
planning/
├── index.md                               # Current active development
├── user-guide.md                          # Human developer commands
├── implementation-guide.md                # This guide
├── 001-dec-cfg-cfg-format-basic-cli/     # Feature folder
│   ├── description.md                    # PM requirements
│   ├── execution-plan-overview.md        # Technical approach
│   ├── execution-plan-steps.md           # Implementation guide
│   └── execution-plan-adrs.md            # Architecture decisions
└── [future-feature-folders]/
```

### Document Types by Feature

#### 1. High-Level Planning (`description.md`)
- Product manager requirements and specifications
- High-level goals and deliverables
- Success criteria

#### 2. Execution Plan Overview (`execution-plan-overview.md`)
- Technical approach and design decisions
- Code examples and architecture
- Package structure and patterns

#### 3. Execution Plan Steps (`execution-plan-steps.md`)
- **Primary implementation guide**
- Step-by-step implementation plan with code examples
- Status tracking for progress

#### 4. Architecture Decision Records (`execution-plan-adrs.md`)
- Stage-specific ADRs numbered as ADR-XXX-YYY
- Technical decisions with context and rationale

## 🔧 Key Files for Implementation

| File | When to Use |
|------|-------------|
| `index.md` | **Start here** - shows current active development |
| `{feature-folder}/execution-plan-steps.md` | **Primary implementation guide** |
| `{feature-folder}/execution-plan-adrs.md` | Context for architectural decisions |
| `{feature-folder}/description.md` | Product manager requirements |

## 🎯 Status Tracking System

### Status Values
- **Not Started** - Step has not been begun
- **In Progress** - Step is currently being worked on  
- **Completed** - Step has been fully implemented and tested
- **Blocked** - Step cannot proceed due to dependencies
- **Skipped** - Step was intentionally skipped for this stage

### Maintenance During Development
1. Update step status as work progresses
2. Add notes in step sections for implementation decisions
3. Update Progress Summary table to reflect current state
4. Mark dependencies as resolved when prerequisites complete

## ⚡ Quick Commands for Progress Tracking

### Check What's Next
1. Open `index.md` to find current active development
2. Follow link to current implementation guide
3. Look at Progress Summary table in execution-plan-steps.md
4. Find first "Not Started" step

### Update Step Status
1. Find the step section (e.g., "## Step 1: Add Verb Constants")
2. Update the "### Status" field
3. Add implementation notes if decisions were made

### Reference Architecture Decisions
1. Look for ADR references in step descriptions (e.g., "see ADR-001-008")
2. Open current feature's ADR file (e.g., `execution-plan-adrs.md`)
3. Search for the specific ADR

## 🧪 Testing Approach

- **Follow test requirements** in each step
- **Test business logic only** - don't test SDKs or libraries
- **Use test-first approach** where specified
- **Integration tests** for command functionality

## ADR Numbering System

Architecture Decision Records use stage-specific numbering:
- Format: `ADR-XXX-YYY` where XXX is stage number, YYY is ADR number
- Example: `ADR-001-008` is the 8th ADR for Stage 1
- Each stage's ADRs start from 001

## Development Transition Process

When completing a development effort:

1. **Complete current development**:
   - Mark all steps as "Completed" in execution-plan-steps.md
   - Ensure all deliverables are implemented and tested

2. **Update index.md**:
   - Change "Current Active Stage" section to next development
   - Update feature name, status, and document links

3. **Prepare new development folder**:
   - Create folder: `planning/NNN-feature-name/`
   - Add description.md with PM requirements
   - Create execution-plan-*.md documents

## 🚨 Important Reminders

- **Dependencies matter**: Don't skip steps with unresolved dependencies
- **Update status consistently**: Keep tracking accurate for future sessions
- **Follow examples exactly**: They're designed to work together
- **Test everything**: Each step includes test requirements
- **Use provided commit messages**: They maintain consistent history

## Step-by-Step Implementation Process

1. **Check dependencies**: Ensure prerequisite steps are completed
2. **Update status**: Mark step as "In Progress"
3. **Implement code**: Follow the detailed implementation guide
4. **Write tests**: Follow test requirements in the step
5. **Commit changes**: Use provided commit message template
6. **Update status**: Mark step as "Completed"
7. **Update summary**: Reflect progress in Progress Summary table

---

**Ready to start? → [index.md](index.md)**