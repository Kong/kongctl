# Stage 7: Testing, Documentation, and Core Improvements - Execution Plan Overview

## Objective
Complete essential testing, documentation, and core improvements for 
declarative configuration management, focusing on production readiness and 
user experience. Additionally, expand imperative command support for resource
parity and implement a complete "Konnect-First" approach across all commands.

## Success Criteria
1. Login command simplified to Konnect-first approach ✓
2. Gateway product renamed to on-prem for clarity
3. Imperative get commands for portals, APIs, and auth strategies
4. All commands support Konnect-first behavior
5. Comprehensive documentation available for users
6. Full integration test coverage for all commands
7. Enhanced error messages and user feedback
8. Migration to public SDK completed
9. Code quality improved through refactoring

## Technical Approach

### 1. Product Clarity and Command Expansion
Establish clear product distinctions and expand command support:
- Rename 'gateway' to 'on-prem' to disambiguate from Konnect gateway
- Add imperative get commands for declarative resources
- Implement Konnect-first pattern across all verbs

### 2. User Experience First
Continue with visible user improvements:
- Create comprehensive documentation
- Improve error messages and feedback
- Add progress indicators for long operations

### 3. Quality Through Testing
Ensure reliability with comprehensive testing:
- Integration tests for all command flows
- Error scenario coverage
- Idempotency verification

### 4. Technical Debt Reduction
Complete technical improvements:
- Migrate to public SDK
- Reduce code duplication
- Improve maintainability

## Implementation Strategy

### Phase 1: Command Infrastructure (Steps 1-6)
Focus on command structure and consistency:
- Login command migration ✓
- Gateway → on-prem rename
- New imperative commands for portals, APIs, auth strategies
- Konnect-first for all verbs

### Phase 2: Documentation (Step 7)
Build comprehensive user documentation:
- Complete declarative configuration guide
- CI/CD integration examples
- Troubleshooting guide

### Phase 3: Quality Assurance (Steps 8-10)
Build comprehensive test coverage:
- Apply command tests
- Sync command tests
- Error scenario tests

### Phase 4: Polish (Steps 11-13)
Enhance user experience:
- Better error messages
- Improved plan display
- Progress indicators

### Phase 5: Technical Completion (Steps 14-15)
Finish technical improvements:
- SDK migration
- Code refactoring

## Risk Mitigation

### Command Changes
- Clear documentation for on-prem rename
- Maintain backward compatibility where possible
- Provide migration guidance

### Backward Compatibility
- Maintain old syntax with deprecation warnings
- Ensure no breaking changes for existing users
- Test migration paths thoroughly

### Testing Coverage
- Start with integration tests to catch real issues
- Focus on user workflows
- Cover error cases comprehensively

### Documentation Quality
- Test all examples before publishing
- Get user feedback on clarity
- Keep documentation in sync with code

## Dependencies
- Completion of Stages 1-6
- Access to Konnect test environment
- Understanding of common user workflows
- Public SDK availability for new resources

## Estimated Timeline
- Phase 1: 4-5 days (command infrastructure)
- Phase 2: 2-3 days (documentation)
- Phase 3: 3-4 days (testing)
- Phase 4: 2-3 days (UX improvements)
- Phase 5: 2-3 days (technical completion)
- Total: ~13-18 days

## Definition of Done
- All commands support Konnect-first pattern
- Clear distinction between on-prem and Konnect resources
- New imperative commands working for all resources
- All tests passing with >80% coverage
- Documentation complete and reviewed
- No internal SDK usage remaining
- User feedback incorporated
- Code quality metrics improved

## Technical Implementation Details

### Gateway → On-Prem Rename
```bash
# Clear product distinction
kongctl get on-prem services        # On-premises Kong Gateway
kongctl get gateway control-planes  # Konnect (via Konnect-first)
kongctl get konnect gateway control-planes  # Explicit Konnect
```

### New Imperative Commands
```bash
# Portal operations
kongctl get portals
kongctl get portal <name>

# API operations
kongctl get apis
kongctl get api <name> --include-versions

# Auth strategy operations
kongctl get auth-strategies
kongctl get auth-strategy <name> --type oauth2
```

### Konnect-First Pattern
All verb commands will default to Konnect when a gateway resource is specified:
- `kongctl get gateway control-planes` → Konnect
- `kongctl list gateway services` → Konnect
- `kongctl create gateway route` → Konnect
- `kongctl delete gateway service` → Konnect

Users can still explicitly specify the product:
- `kongctl get konnect gateway control-planes`
- `kongctl get on-prem services`