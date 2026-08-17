// Package web contains the Stellar templates and browser assets.
//
// Templates and browser assets are owned by the Stellar web boundary. Shared
// configuration and view data remain in internal/settings/viewmodel.
package web

import "embed"

// FS contains the resources shipped with the Stellar module.
//
//go:embed templates all:assets
var FS embed.FS
