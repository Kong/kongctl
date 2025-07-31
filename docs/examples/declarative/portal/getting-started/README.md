# Getting Started Portal Example

This example demonstrates a complete Kong Developer Portal setup using declarative configuration. 
It includes a fully configured portal with pages, customization, and reusable snippets.

## Overview

This example creates:
- A developer portal with authentication disabled and public visibility
- Portal customization with theme colors and navigation menus
- A hierarchy of pages including home, APIs, getting started, and guides
- Reusable snippets for common UI components

## Structure

```
getting-started/
├── portal.yaml          # Main declarative configuration
├── pages/              # Page content files
│   ├── home.md
│   ├── apis.md
│   ├── getting-started.md
│   ├── guides.md
│   └── guides/
│       ├── document-apis.md
│       ├── publish-apis.md
│       └── publish-apis/
│           └── versioning.md
└── snippets/           # Reusable content snippets
    ├── example-guides-page-banner.md
    ├── example-guides-page-header.md
    ├── example-guides-page-nav.md
    ├── example-hero-image.md
    ├── example-logo-bar.md
    └── example-page-toc.md
```

## Key Features

### Portal Configuration
- **Authentication**: Disabled for public access
- **RBAC**: Disabled
- **Auto-approval**: Disabled for both developers and applications
- **Default visibility**: All APIs and pages are public by default

### Nested Structure
This example uses a nested configuration where all portal resources are defined within the portal definition:
- `customization` - Theme, layout, and navigation menus
- `pages` - All portal pages with hierarchical relationships
- `snippets` - Reusable content components

### Kongctl Metadata
The portal resource (parent) can include a `kongctl` section for tool-specific settings:
- `protected` - Prevents accidental deletion via sync
- `namespace` - Resource ownership for multi-team environments

**Note**: Child resources (pages, customization, snippets) do not support kongctl metadata - they inherit settings from their parent portal.

### Theme Customization
- **Primary color**: #8250FF
- **Layout**: Top navigation (topnav)
- **Navigation menus**: Main menu and footer sections configured

### Page Hierarchy
The example demonstrates parent-child page relationships using nested `children`:
```yaml
pages:
  - ref: guides
    children:
      - ref: guides-document-apis
      - ref: guides-publish-apis
        children:
          - ref: guides-versioning
```

### Content Features
- **Markdown content**: All pages use markdown with custom components
- **File references**: Content is loaded from external files using `!File` tag
- **Reusable snippets**: Common UI components can be referenced across pages

## Usage

To apply this configuration:

```bash
kongctl apply -f docs/examples/declarative/portal/getting-started/portal.yaml
```

## Custom Components

The portal pages use special markdown components:
- `::page-section` - Creates page sections with styling
- `::page-hero` - Hero sections with background colors
- `::button` - Styled buttons with links
- `::snippet` - References to reusable snippets
- `::page-layout` - Page layout configuration

## Notes

- All pages have `visibility: "public"` and `status: "published"`
- The `!File` tag is used to load content from external files
- Parent-child relationships are defined using `parent_page_ref`
- Snippets can be referenced in pages using the `::snippet` component
