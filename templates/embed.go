package templates

import "embed"

// FS is the embedded filesystem for templates
//go:embed *.html layout.html
var FS embed.FS
