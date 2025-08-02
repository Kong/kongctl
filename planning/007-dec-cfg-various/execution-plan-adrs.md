# Stage 7: Testing, Documentation, and Core Improvements - Architecture Decision Records

## ADR-007-001: Konnect-First Login Command

### Status
Accepted (Implemented)

### Context
Currently, users must type `kongctl login konnect` to authenticate. Since 
kongctl is primarily focused on Konnect (with future gateway support), 
requiring the product specification adds unnecessary friction.

### Decision
Make login command work directly as `kongctl login` without requiring product 
specification. Maintain backward compatibility with deprecation warning.

### Consequences
- **Positive**: Simpler user experience, aligned with Konnect-first philosophy
- **Positive**: Sets precedent for future command simplification
- **Negative**: Requires deprecation period and user communication
- **Negative**: May confuse users during transition period

### Implementation Notes
- Add `--product` flag for future extensibility
- Show clear deprecation message for old syntax
- Update all documentation and examples

---

## ADR-007-002: Gateway to On-Prem Product Rename

### Status
Proposed

### Context
The term "gateway" is ambiguous in kongctl - it could refer to Konnect-hosted 
gateway resources or on-premises Kong Gateway. This ambiguity prevents us from 
implementing a clean Konnect-first approach for gateway resources.

### Decision
Rename the 'gateway' product to 'on-prem' to clearly distinguish between:
- `kongctl get gateway control-planes` - Defaults to Konnect via Konnect-first
- `kongctl get on-prem services` - Explicitly for on-premises Kong Gateway

### Consequences
- **Positive**: Clear mental model for users
- **Positive**: Enables Konnect-first for gateway resources
- **Positive**: Consistent with product positioning
- **Negative**: Breaking change for existing on-prem commands
- **Negative**: Requires documentation updates
- **Negative**: May need future revision based on product strategy

### Implementation Notes
- Add comment noting this naming may change
- Update all i18n keys and examples
- Consider compatibility period if on-prem commands are in use

---

## ADR-007-003: Imperative Command Resource Parity

### Status
Proposed

### Context
Declarative configuration supports portals, APIs, and auth strategies, but 
imperative commands only support gateway resources. This creates an inconsistent 
experience where users can manage resources declaratively but not imperatively.

### Decision
Implement imperative `get` commands for all resources supported by declarative 
configuration: portals, APIs, and auth strategies.

### Consequences
- **Positive**: Consistent experience across command types
- **Positive**: Users can inspect resources before declarative operations
- **Positive**: Foundation for future CRUD operations
- **Negative**: Increased implementation effort
- **Negative**: More commands to maintain and document

### Implementation Notes
- Start with read-only `get` operations
- Follow existing patterns from control-planes
- Support same output formats (text, json, yaml)
- Consider adding list/create/delete in future phases

---

## ADR-007-004: Complete Konnect-First for All Commands

### Status
Proposed

### Context
Declarative commands (apply, sync, plan, diff, export) already implement 
Konnect-first behavior. Login is being updated. Imperative commands (get, list, 
create, delete) still require explicit product specification.

### Decision
Apply Konnect-first pattern to ALL verb commands, making Konnect the default 
product whenever ambiguous.

### Consequences
- **Positive**: Consistent behavior across entire CLI
- **Positive**: Simpler commands for common use case
- **Positive**: Better new user experience
- **Negative**: On-prem users must be more explicit
- **Negative**: Risk of user confusion during transition
- **Negative**: More complex command implementation

### Implementation Notes
- Follow pattern established by declarative commands
- Ensure backward compatibility
- Update all help text to show new patterns
- Consider `--product` flag for explicit control

---

## ADR-007-005: Integration Test Strategy

### Status
Proposed

### Context
Declarative configuration is a complex feature with many edge cases. We need 
comprehensive testing to ensure reliability and catch regressions.

### Decision
Create integration tests that test full workflows against real Konnect APIs, 
organized by command (apply, sync) and scenario (errors, edge cases).

### Consequences
- **Positive**: High confidence in feature reliability
- **Positive**: Catches real API integration issues
- **Positive**: Documents expected behavior through tests
- **Negative**: Slower test execution than unit tests
- **Negative**: Requires test environment maintenance

### Implementation Notes
- Use build tags to separate integration tests
- Create shared test utilities for common operations
- Ensure tests are idempotent and can run in parallel

---

## ADR-007-006: Error Message Enhancement Strategy

### Status
Proposed

### Context
Current error messages often lack context, making it difficult for users to 
understand what went wrong and how to fix it.

### Decision
Enhance all error messages to include resource context, actionable hints, and 
consistent formatting throughout the declarative configuration system.

### Consequences
- **Positive**: Better user experience during error scenarios
- **Positive**: Reduced support burden
- **Positive**: Easier debugging for users
- **Negative**: Requires systematic review of all error paths
- **Negative**: May increase code verbosity

### Implementation Notes
- Create error wrapping utilities for consistent formatting
- Add hints for common error scenarios
- Include resource identifiers in all error messages

---

## ADR-007-007: Documentation Structure

### Status
Proposed

### Context
Declarative configuration is a major feature that needs comprehensive 
documentation covering concepts, usage, examples, and troubleshooting.

### Decision
Create a dedicated documentation structure with concept guides, reference 
documentation, examples, and troubleshooting guides.

### Consequences
- **Positive**: Users can find information easily
- **Positive**: Reduces learning curve
- **Positive**: Provides clear migration path
- **Negative**: Requires significant documentation effort
- **Negative**: Must be maintained as features evolve

### Implementation Notes
- Test all examples before publishing
- Include real-world scenarios
- Provide clear comparisons between apply and sync

---

## ADR-007-008: Public SDK Migration Approach

### Status
Proposed

### Context
Some commands still use the internal Konnect SDK. We need to complete the 
migration to the public SDK for consistency and maintainability.

### Decision
Migrate remaining commands (particularly dump) to use the public SDK, removing 
all internal SDK dependencies.

### Consequences
- **Positive**: Single SDK dependency simplifies maintenance
- **Positive**: Aligns with Kong's public API strategy
- **Positive**: Better stability guarantees
- **Negative**: May lose access to some internal-only features
- **Negative**: Requires careful testing of migrated functionality

### Implementation Notes
- Map internal API calls to public equivalents
- Ensure feature parity after migration
- Update imports and dependencies

---

## ADR-007-009: Code Quality Standards

### Status
Proposed

### Context
As the codebase grows, maintaining code quality becomes increasingly important. 
We need standards for code organization, testing, and documentation.

### Decision
Establish and enforce code quality standards including >80% test coverage, 
reduced duplication, consistent error handling, and clear function design.

### Consequences
- **Positive**: More maintainable codebase
- **Positive**: Easier onboarding for new contributors
- **Positive**: Fewer bugs and regressions
- **Negative**: Requires refactoring effort
- **Negative**: May slow initial development

### Implementation Notes
- Use linting tools to enforce standards
- Create shared utilities for common patterns
- Break down complex functions systematically

---

## ADR-007-010: Progress Reporting Design

### Status
Proposed

### Context
Long-running operations (apply/sync with many resources) provide no feedback 
during execution, leaving users uncertain about progress.

### Decision
Implement a progress reporting system that shows operation status for 
long-running commands without cluttering output for fast operations.

### Consequences
- **Positive**: Better user experience for long operations
- **Positive**: Users can see if operation is stuck
- **Positive**: Clearer indication of what's happening
- **Negative**: Adds complexity to executor flow
- **Negative**: Must handle different output formats

### Implementation Notes
- Only show progress for operations >2 seconds
- Ensure compatibility with JSON/YAML output
- Keep progress updates on single line when possible