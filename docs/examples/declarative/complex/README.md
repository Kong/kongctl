# Complex Declarative Configuration Examples

This directory contains more sophisticated examples that demonstrate advanced use cases and patterns in Kong declarative configuration. Each example is in its own subdirectory.

## Available Examples

### API Lifecycle Example
```bash
kongctl plan --dir docs/examples/declarative/complex/api-lifecycle-example
```
Demonstrates managing multiple API versions through their lifecycle (deprecated, current, beta) with multiple portal publications.

### Full Portal Setup Example
```bash
kongctl plan --dir docs/examples/declarative/complex/full-portal-setup-example
```
Complete portal setup with multiple portals, authentication strategies, APIs, and cross-references.

### Multi-Resource Example
```bash
kongctl plan --dir docs/examples/declarative/complex/multi-resource-example
```
Shows a comprehensive configuration with all resource types working together.

## Key Concepts Demonstrated

- **API Lifecycle Management**: Multiple versions, deprecation, sunset dates
- **Portal Management**: Different portal types (public, partner, internal)
- **Authentication Strategies**: Multiple auth types and configurations
- **Cross-Resource References**: How resources reference each other
- **Production Patterns**: Realistic configurations for production use

## Structure

Each example subdirectory contains:
- One YAML file with a complete, self-contained configuration
- All necessary dependencies within the same file
- Production-ready patterns and best practices
- Detailed comments explaining advanced concepts

These examples are designed to be:
- **Comprehensive**: Show real-world complexity
- **Self-contained**: All resources and dependencies included
- **Production-oriented**: Patterns suitable for actual deployments