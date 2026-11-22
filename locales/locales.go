// Package locales embeds the app's message catalogs. Ids use the prefixes app., nav., col., library., status., error. and toast.;
// "shell." and "command." belong to the kit.
package locales

import "embed"

// FS holds en.json and de.json.
//
//go:embed en.json de.json
var FS embed.FS

// Files lists the catalog paths inside FS.
var Files = []string{"en.json", "de.json"}
