# External Resources Implementation Steps

## Progress Summary

| Step | Task | Status | Notes |
|------|------|--------|-------|
| 1 | Schema and Configuration | ✅ Completed | Resolution naming theme |
| 2 | Resource Type Registry | 🚧 In Progress | Foundation complete (2025-08-07) |
| 3 | External Resource Resolver | Not Started | |
| 4 | Reference Resolution Integration | Not Started | |
| 5 | Error Handling | Not Started | |
| 6 | Integration with Planning | Not Started | |
| 7 | Testing | Not Started | |
| 8 | Documentation | Not Started | |

**Overall Progress**: 2/8 steps (25%) - Step 2 foundation complete, ready for full adapter implementations

## Phase 1: Core Implementation

### Step 1: Schema and Configuration ✅ COMPLETED
- [x] Define external_resources schema in configuration types
- [x] Add validation for external resource blocks
- [x] Support both direct ID and selector patterns
- [x] Add parent field support for hierarchical resources

**Implementation Notes**:
- Implemented with "Resolution" naming theme to avoid stuttering
- External resources do not have Kongctl metadata (cannot be protected/namespaced)
- Registry expanded to include all portal and API child resource types
- Complete validation framework with XOR validation for ID/selector

### Step 2: Resource Type Registry 🚧 IN PROGRESS
- [x] Create registry for supported external resource types
- [x] Map resource types to SDK operations (foundation complete)
- [x] Define parent-child relationships
- [ ] Add resource type validation

**Implementation Notes** (2025-08-07):
- Created base adapter pattern with common filtering and validation logic
- Implemented adapter factory with dependency injection for all 13 resource types
- Extended state client with GetPortalByID and ListPortalsWithFilter methods
- Added InjectAdapters method to registry for runtime adapter injection
- Portal and API adapters fully functional, others have TODO stubs
- Added support for ce_service (core entity) with control_plane parent requirement
- Comprehensive test coverage for base adapter functionality

### Step 3: External Resource Resolver
- [ ] Implement ExternalResourceResolver struct
- [ ] Parse external_resources from configuration
- [ ] Build dependency graph for resolution order
- [ ] Implement direct ID resolution
- [ ] Implement matchFields selector logic
- [ ] Add SDK query execution
- [ ] Implement match validation (exactly one)
- [ ] Add resource caching mechanism

### Step 4: Reference Resolution
- [ ] Implement ReferenceResolver for dependency handling
- [ ] Detect external resource references in configurations
- [ ] Implement implicit ID resolution for _id fields
- [ ] Handle mixed internal/external references
- [ ] Add reference validation

### Step 5: Error Handling
- [ ] Implement clear error messages for zero matches
- [ ] Implement error messages for multiple matches
- [ ] Add validation errors for invalid configurations
- [ ] Handle SDK errors gracefully
- [ ] Add detailed error context

### Step 6: Integration with Planning
- [ ] Integrate external resolution into planning phase
- [ ] Ensure resolution happens before plan generation
- [ ] Update plan output to show external resources
- [ ] Add external resource status to plan

### Step 7: Testing
- [ ] Unit tests for resolver components
- [ ] Integration tests with mock SDK responses
- [ ] Test error scenarios (0 matches, multiple matches)
- [ ] Test parent-child resolution
- [ ] Test implicit ID resolution
- [ ] End-to-end tests with real resources

### Step 8: Documentation
- [ ] User guide for external resources
- [ ] Migration guide from other tools
- [ ] Configuration examples
- [ ] Troubleshooting guide

## Phase 2: Extended Support (Future)

### Step 9: Additional Resource Types
- [ ] Add support for all SDK-supported resource types
- [ ] Implement resource-specific validation
- [ ] Add complex parent relationships

### Step 10: matchExpressions Support
- [ ] Design operator system
- [ ] Implement comparison operators
- [ ] Implement string operators
- [ ] Implement array operators
- [ ] Add operator validation

### Step 11: Performance Optimization
- [ ] Batch SDK calls where possible
- [ ] Implement parallel resolution
- [ ] Optimize caching strategy
- [ ] Add resolution metrics

## Testing Strategy

### Unit Tests
- Schema validation
- Selector matching logic
- Reference resolution
- Error handling

### Integration Tests
- SDK integration
- Parent-child resolution
- Mixed references
- Planning integration

### E2E Tests
- Real Konnect resources
- Migration scenarios
- Complex dependencies
- Error scenarios

## Rollout Plan

### Alpha Release
- Internal testing
- Limited resource types
- Basic documentation

### Beta Release
- Customer preview
- Core resource types
- Full documentation
- Migration examples

### GA Release
- All planned resource types
- Performance optimized
- Production ready
- Complete documentation

## Risk Mitigation

### Risk: SDK Changes
**Mitigation**: Abstract SDK operations behind interfaces

### Risk: Performance Issues
**Mitigation**: Implement caching and batch operations early

### Risk: Complex Selectors
**Mitigation**: Start with simple matchFields, extend gradually

### Risk: Breaking Changes
**Mitigation**: Design extensible schema from the start

## Success Criteria

- [ ] External resources resolve correctly
- [ ] Clear error messages for all failure cases
- [ ] No performance regression
- [ ] Positive user feedback
- [ ] Successful migration stories

## Dependencies

- Konnect Go SDK
- Existing resource type system
- Planning phase implementation
- Configuration management system

## Timeline

Implementation will proceed through the phases as prioritized, with Phase 1 
focusing on core functionality and Phase 2 adding extended capabilities.