# Basic Declarative Configuration Examples

This directory contains standalone examples that demonstrate individual concepts in Kong declarative configuration. Each example is in its own subdirectory and can be run independently.

## Available Examples

### Portal Example
```bash
kongctl plan --dir docs/examples/declarative/basic/portal-example
```
Demonstrates basic portal configuration with customization options.

### Authentication Strategy Example  
```bash
kongctl plan --dir docs/examples/declarative/basic/auth-strategy-example
```
Shows different types of authentication strategies (OAuth, API Key).

### Control Plane Example
```bash
kongctl plan --dir docs/examples/declarative/basic/control-plane-example
```
Examples of control plane definitions for different environments.

### API with Children Example
```bash
kongctl plan --dir docs/examples/declarative/basic/api-with-children-example
```
Complete example showing API with nested versions, publications, and implementations. This is a self-contained example that includes all required dependencies (portals, auth strategies, control planes).

## Structure

Each example subdirectory contains:
- One or more YAML files with Kong declarative configuration
- Self-contained resources (all references are satisfied within the directory)
- Comments explaining the key concepts

These examples are designed to be:
- **Standalone**: Can be run independently without external dependencies
- **Educational**: Well-commented to explain concepts
- **Functional**: Will load successfully with kongctl plan command