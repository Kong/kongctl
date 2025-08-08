# External Resources Implementation Steps

## Progress Summary

| Step | Task | Status | Notes |
|------|------|--------|-------|
| 1 | Schema and Configuration | ✅ Completed | Resolution naming theme |
| 2 | Resource Type Registry | ✅ Completed | All 13 adapters implemented (2025-08-07) |
| 3 | External Resource Resolver | ✅ Completed | Core resolver, dependency graph, planner integration (2025-08-08) |
| 4 | Reference Resolution Integration | ✅ Completed | Dynamic field detection (2025-08-08) |
| 5 | Error Handling | Not Started | |
| 6 | Integration with Planning | Not Started | |
| 7 | Testing | Not Started | |
| 8 | Documentation | Not Started | |

**Overall Progress**: 4/8 steps (50%) - Step 4 complete with dynamic reference resolution

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

### Step 2: Resource Type Registry ✅ COMPLETED
- [x] Create registry for supported external resource types
- [x] Map resource types to SDK operations (all 13 adapters)
- [x] Define parent-child relationships
- [x] Add resource type validation

**Implementation Notes** (2025-08-07):
- Created base adapter pattern with common filtering and validation logic
- Implemented adapter factory with dependency injection for all 13 resource types
- Extended state client with resolution methods for all resource types:
  - Top-level: portal, api, control_plane, application_auth_strategy
  - Child resources: ce_service (with control_plane parent)
  - Portal children: customization, custom_domain, page, snippet
  - API children: version, implementation, document, publication
- Added InjectAdapters method to registry for runtime adapter injection
- All 13 adapters fully implemented with GetByID and GetBySelector methods
- Fixed SDK integration issues (union types, response field mappings)
- Comprehensive test coverage for base adapter functionality
- All quality checks passing (build, lint, test)

### Step 3: External Resource Resolver ✅ COMPLETED
- [x] Implement ExternalResourceResolver struct
- [x] Parse external_resources from configuration
- [x] Build dependency graph for resolution order
- [x] Implement direct ID resolution
- [x] Implement matchFields selector logic
- [x] Add SDK query execution (via adapters)
- [x] Implement match validation (exactly one)
- [x] Add resource caching mechanism

**Implementation Notes** (2025-08-08):
- Created ExternalResourceResolver with full resolution workflow
- Implemented dependency graph with topological sorting (Kahn's algorithm)
- Added interface-based design to avoid circular dependencies
- Integrated with planner for pre-resolution phase
- Enhanced reference resolver to check external resource cache
- All quality gates passing (build, tests)
- 4 minor linting style warnings about naming conventions (kept for consistency)

### Step 4: Reference Resolution Integration ✅ COMPLETED
- [x] Replace hardcoded reference field detection with dynamic approach
- [x] Leverage resource GetReferenceFieldMappings() interface
- [x] Add caching layer for performance optimization
- [x] Maintain backward compatibility with fallback approach
- [x] Comprehensive test coverage for dynamic resolution

**Implementation Notes** (2025-08-08):
- Replaced hardcoded isReferenceField() with dynamic resource mapping queries
- Added getResourceMappings() with thread-safe caching
- Created isReferenceFieldDynamic() and getResourceTypeForFieldDynamic()
- Resources now define their own reference fields via GetReferenceFieldMappings()
- Backward compatibility maintained - both approaches coexist
- Full test coverage including edge cases and performance validation
- All quality gates passing (build, lint, test)

### Step 5: Error Handling ✅ COMPLETED
- [x] Implement clear error messages for zero matches
- [x] Implement error messages for multiple matches
- [x] Add validation errors for invalid configurations
- [x] Handle SDK errors gracefully
- [x] Add detailed error context

**Implementation Notes**:
- Created three structured error types: ResourceValidationError, ResourceResolutionError, ResourceSDKError
- Enhanced zero match errors with available resources context and suggestions
- Enhanced multiple match errors with resource details and disambiguation fields
- Added SDK error classification system (Network, Auth, Validation, etc.)
- Implemented user-friendly error translation with actionable suggestions
- Enhanced field-specific validation with detailed error messages
- Added registry integration methods for error context
- All quality gates passing (build, lint)

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

- [x] External resources resolve correctly (Steps 1-4 complete)
- [x] Clear error messages for all failure cases (Step 5 complete)
- [ ] No performance regression (Step 6 in progress)
- [ ] Positive user feedback (pending full implementation)
- [ ] Successful migration stories (pending full implementation)

## Dependencies

- Konnect Go SDK
- Existing resource type system
- Planning phase implementation
- Configuration management system

## Timeline

Implementation will proceed through the phases as prioritized, with Phase 1 
focusing on core functionality and Phase 2 adding extended capabilities.