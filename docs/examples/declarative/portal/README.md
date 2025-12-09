# Getting Started Portal Example

This example demonstrates a complete Kong Developer Portal setup using declarative configuration. 
It includes a fully configured portal with APIs, pages, customizations, and reusable snippets.

## Overview

This example creates:
- A developer portal with authentication disabled and public visibility
- APIs published to the portal
- Portal assets including logo and favicon
- Portal customization with theme colors and navigation menus
- A hierarchy of pages including home, APIs, getting started, and guides
- Reusable snippets for common UI components

## Structure

```
portal.yaml         # Portal definition
apis.yaml           # API definitions
assets/             # Portal asset files
├── logo.svg        # Portal logo
└── favicon.svg     # Portal favicon
pages/              # Page content files
├── home.md
├── apis.md
├── getting-started.md
├── guides.md
└── guides/
    ├── document-apis.md
    ├── publish-apis.md
    └── publish-apis/
        └── versioning.md
snippets/           # Reusable content snippets
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

### Portal Assets
Portal assets (logo and favicon) are managed using the `!file` tag to load image
files:
```yaml
assets:
  logo: !file ./assets/logo.svg
  favicon: !file ./assets/favicon.svg
```

The `!file` tag automatically:
- Detects image files by extension (.png, .jpg, .svg, .ico)
- Converts binary images to base64-encoded data URLs
- Handles the proper MIME type for each format
- Supports up to 10MB per file

Supported formats: PNG, JPEG, SVG, ICO

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

To sync this configuration:

```bash
kongctl sync -f portal.yaml -f apis.yaml
```

If all changes are applied, you have successfully created a developer portal with APIs and pages.
Assuming you have the `jq` command-line tool installed, you can obtain the portal URL with:

```bash
kongctl get portals "My First Portal" -o json | jq -r '"https://\(.default_domain)"'
```

