# Claude Code Implementation Guide

**Quick start guide for implementing declarative configuration features using these planning documents.**

## 🚀 Getting Started

### 1. Check Current Status
```markdown
Read: 001-execution-plan-steps.md → Progress Summary table
```
This shows you exactly what's implemented and what's next.

### 2. Find Your Next Task
Look for the first step with:
- Status: "Not Started" 
- Dependencies: All marked "Completed" or "None"

### 3. Understand the Context
- **Step details**: Full implementation guidance in the step section
- **Architecture context**: Reference ADRs mentioned in the step
- **Code examples**: Provided in each step for exact implementation

### 4. Update Progress
- Mark step as "In Progress" before starting
- Add implementation notes to the step if you make decisions
- Mark as "Completed" when done (including tests)
- Update Progress Summary table

## 📋 Current Implementation Status

**Stage 1: Configuration Format & Basic CLI**
- **Status**: Ready for implementation 
- **Next step**: Step 1 - Add Verb Constants
- **Location**: [001-execution-plan-steps.md](001-execution-plan-steps.md)

## 🎯 Implementation Workflow

1. **Update status** → "In Progress"
2. **Read step details** → Complete implementation guide provided
3. **Write code** → Follow examples exactly
4. **Write tests** → Test requirements specified in each step
5. **Commit** → Use commit message template provided
6. **Update status** → "Completed"
7. **Move to next step**

## 🔧 Key Files for Implementation

| File | When to Use |
|------|-------------|
| `001-execution-plan-steps.md` | **Primary implementation guide** - start here |
| `001-execution-plan-adrs.md` | When you need context for why decisions were made |
| `001-execution-plan-overview.md` | When you need to understand overall architecture |
| `process.md` | When you need to understand the development process |

## ⚡ Quick Commands for Progress Tracking

### Check What's Next
1. Open `001-execution-plan-steps.md`
2. Look at Progress Summary table
3. Find first "Not Started" step with no blocking dependencies

### Update Step Status
1. Find the step section (e.g., "## Step 1: Add Verb Constants")
2. Update the "### Status" field
3. Add notes in the step if you make implementation decisions

### Reference Architecture Decisions
1. Look for ADR references in step descriptions (e.g., "see ADR-001-008")
2. Open `001-execution-plan-adrs.md` 
3. Search for the specific ADR (e.g., "ADR-001-008")

## 🧪 Testing Approach

- **Follow test requirements** in each step
- **Test business logic only** - don't test SDKs or libraries
- **Use test-first approach** where specified
- **Integration tests** for command functionality

## 📝 Notes and Decisions

- **Add implementation notes** directly to step sections
- **Update commit messages** if you deviate from templates
- **Reference ADRs** when making related decisions
- **Keep Progress Summary updated** for visibility

## 🚨 Important Reminders

- **Dependencies matter**: Don't skip steps with unresolved dependencies
- **Update status consistently**: Keep tracking accurate for future sessions
- **Follow examples exactly**: They're designed to work together
- **Test everything**: Each step includes test requirements
- **Use provided commit messages**: They maintain consistent history

---

**Ready to start? → [001-execution-plan-steps.md](001-execution-plan-steps.md)**