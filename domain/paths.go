package domain

import "strings"

// OfficeDir is the folder that holds sanity.json, relative to the project root.
const OfficeDir = "office"

// OfficeRel turns a path relative to the project root into one relative to the
// office folder, the way a user in office/ would type it. "receipts/2026/x.pdf"
// becomes "../receipts/2026/x.pdf", "office/a.pdf" becomes "a.pdf". Backslashes
// count as separators; the result always uses "/". Stored paths and the path
// template stay relative to the project root.
func OfficeRel(projectRel string) string {
	p := strings.ReplaceAll(projectRel, `\`, "/")
	if rest, ok := strings.CutPrefix(p, OfficeDir+"/"); ok {
		return rest
	}
	return "../" + strings.TrimPrefix(p, "./")
}
