// Package assets embeds the bundled themes (templates, CSS, and fonts) so the
// mdpng binary is fully self-contained.
package assets

import "embed"

//go:embed all:themes
var FS embed.FS
