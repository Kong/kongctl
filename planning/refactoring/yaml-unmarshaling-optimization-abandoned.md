# YAML Double-Unmarshaling Optimization (ABANDONED)

**Status:** ABANDONED - This refactoring was not completed  
**Date:** 2025-08-14  
**Reason:** Technical complexity outweighed benefits; multiple approaches still required double unmarshaling

## Problem Identified

The current YAML processing pipeline performs double unmarshaling, causing performance degradation:

1. **First unmarshal** in `internal/declarative/tags/resolver.go:Process()`:
   - `yaml.Unmarshal(data, &doc)` - unmarshals to `yaml.Node`
   - Processes custom tags (`!file`, etc.) by walking Node tree
   - `encoder.Encode(&doc)` - marshals processed Node back to bytes

2. **Second unmarshal** in `internal/declarative/loader/loader.go:165`:
   - `yaml.UnmarshalStrict(content, &temp)` - unmarshals bytes to Go structs

This creates an unnecessary marshal/unmarshal round-trip for large configurations.

## Key Discovery

During analysis, we confirmed that YAML tags (`!file`) only substitute VALUES for existing fields - they do NOT add new field names to the structure. This was verified by examining test files showing patterns like:

```yaml
name: !file portal-name.txt                    # replaces VALUE of existing 'name' field
description: !file ./external.yaml#description # replaces VALUE of existing 'description' field  
display_name: !file ./external.yaml#version    # replaces VALUE of existing 'display_name' field
```

## Attempted Solutions

### Approach 1: Pre-validation + ProcessToNode
- Validate YAML structure first with `yaml.UnmarshalStrict`
- Process tags with new `ProcessToNode` method returning `yaml.Node`
- Decode processed node directly

**Problem:** Still requires two unmarshals (validation + tag processing)

### Approach 2: Node-based Strict Validation
- Single unmarshal to `yaml.Node`
- Implement custom strict validation by walking node tree
- Process tags on same node
- Decode to struct

**Problem:** Complex implementation requiring custom strict validation logic equivalent to `yaml.UnmarshalStrict`

### Approach 3: Struct-based Tag Processing
- Single `yaml.UnmarshalStrict` to Go structs
- Process tags by walking struct fields
- No second unmarshal needed

**Problem:** Would require complete rewrite of tag processing system from Node-based to struct-based

## Technical Challenges

1. **Strict Validation Complexity**: Implementing equivalent behavior to `yaml.UnmarshalStrict` for `yaml.Node` would require:
   - Reflection-based struct field analysis
   - Node tree walking with field validation
   - Identical error message formatting including field suggestions

2. **Error Message Preservation**: Must maintain exact error messages including the `suggestFieldName` functionality that suggests corrections for typos

3. **Performance vs Complexity Trade-off**: The optimization would improve performance but significantly increase codebase complexity

## Files Analyzed

- `internal/declarative/tags/resolver.go` - Tag processing implementation
- `internal/declarative/tags/file.go` - File tag resolver showing value substitution only
- `internal/declarative/loader/loader.go` - Current double-unmarshal location
- Test files showing tag usage patterns confirming no new fields added

## Conclusion

While the double unmarshaling issue is real and impacts performance, the complexity of the proposed solutions outweighs the benefits. The optimization would require either:

1. Maintaining double unmarshaling in a different form
2. Implementing complex custom validation logic 
3. Major architectural changes to tag processing

**Recommendation:** Focus on other refactoring opportunities that provide clearer benefits with lower implementation complexity.

## Research Value

This investigation provided valuable insights:
- Confirmed YAML tags only substitute values, never add fields
- Identified the exact performance bottleneck in YAML processing
- Explored multiple optimization approaches and their trade-offs
- Documented the current tag processing architecture

This information may be useful for future optimizations or architectural decisions.