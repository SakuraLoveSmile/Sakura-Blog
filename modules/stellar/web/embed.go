// Package web contains the Stellar templates and browser assets.
//
// The legacy internal/template package still owns the compatibility model and
// rendering API during this migration. Keeping the canonical resource tree
// here lets the next extraction move only Go view models, without moving files
// or changing the deployed HTML at the same time.
package web

import "embed"

// FS contains the resources shipped with the Stellar module.
//
//go:embed templates all:assets
var FS embed.FS
