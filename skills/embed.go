package skills

import "embed"

// BundledFS contains the built-in kongctl skills distributed with the CLI.
//
//go:embed kongctl-declarative kongctl-extension-builder
var BundledFS embed.FS
