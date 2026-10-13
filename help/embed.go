// Package help holds the SPEC §16 help pages. The pages are data shared by any implementation;
// this package only embeds them.
package help

import "embed"

// Pages holds every help/*.md file.
//
//go:embed *.md
var Pages embed.FS
