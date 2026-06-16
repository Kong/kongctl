# Getting Started Portal Example

This example demonstrates a complete Kong Developer Portal setup using
declarative configuration. It includes a fully configured portal with APIs,
pages, customizations, and reusable snippets.

## Overview

This example creates:
- A developer portal with authentication disabled and public visibility
- APIs published to the portal
- Portal assets including logo and favicon
- An optional commented portal IP allow list for trusted network access
- Portal integrations for Google Tag Manager and Google Analytics 4
- Portal customization with theme colors and navigation menus
- A hierarchy of pages including home, APIs, getting started, and guides
- Reusable snippets for common UI components

## Structure

```
portal.yaml         # Portal definition
apis.yaml           # API definitions
assets/             # Portal asset files
├── logo.png        # Portal logo
└── favicon.png     # Portal favicon
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
Portal logo and favicon images are declared under the portal `assets` block.
Use relative paths with the `!file` tag; paths are resolved from the YAML file
that contains the tag. For this example, the image files live in `assets/`
next to `portal.yaml`:

```yaml
assets:
  logo: !file ./assets/logo.png
  favicon: !file ./assets/favicon.png
```

The `!file` tag automatically:
- Loads files from inside the configuration base directory
- Converts image files to base64-encoded data URLs
- Handles the MIME type for each supported image format
- Supports up to 10MB per file

Supported portal asset formats include PNG, JPEG, SVG, ICO, and GIF.

After syncing the example, verify or export the assets with:

```bash
kongctl get portal assets logo --portal-name "My First Portal"
kongctl get portal assets logo --portal-name "My First Portal" \
  --output-file my-logo.png
kongctl get portal assets favicon --portal-name "My First Portal" \
  --output-file my-favicon.ico
```

### Portal IP Allow List
Portal IP allow lists can be configured as a singleton child of a portal. The
example in `portal.yaml` is commented out so the public getting started portal
remains browsable after applying the example. Uncomment it and replace
`allowed_ips` with trusted individual IP addresses or CIDR blocks:

```yaml
ip_allow_list:
  ref: getting-started-ip-allow-list
  allowed_ips:
    - 198.51.100.10
    - 203.0.113.0/24
```

The same resource can also be declared at the top level with
`portal_ip_allow_lists` when keeping child resources separate from the portal
definition.

### Portal Integrations
Portal integrations are configured as a singleton child of the portal. This
example includes disabled Google Tag Manager and Google Analytics 4 integrations
with placeholder IDs:

```yaml
integrations:
  ref: getting-started-integrations
  google_tag_manager:
    enabled: false
    config_data:
      id: GTM-EXAMPLE
  google_analytics_4:
    enabled: false
    config_data:
      id: G-EXAMPLE
```

### Theme Customization
- **Primary color**: #8250FF
- **Layout**: Top navigation (topnav)
- **Navigation menus**: Main menu and footer sections configured

### Page Hierarchy
The example demonstrates parent-child page relationships using nested
`children`:
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

If all changes are applied, you have successfully created a developer portal
with APIs and pages. Assuming you have the `jq` command-line tool installed,
you can obtain the portal URL with:

```bash
kongctl get portals "My First Portal" -o json \
  | jq -r '"https://\(.default_domain)"'
```
