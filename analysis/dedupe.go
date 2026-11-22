package analysis

import (
	"strings"

	"github.com/72nd/sanity/domain"
)

// FindDuplicate looks for an already filed copy: same file hash, same text
// hash, or same invoice number within the same type. The reason is a short
// English description for the UI layer to map.
func FindDuplicate(entries []domain.LibraryEntry, sha, textHash, typeID, invoice string) (domain.LibraryEntry, string, bool) {
	if sha != "" {
		for _, e := range entries {
			if e.SHA256 == sha {
				return e, "same file (SHA-256)", true
			}
		}
	}
	if textHash != "" {
		for _, e := range entries {
			if e.TextHash == textHash {
				return e, "same text", true
			}
		}
	}
	inv := strings.ToLower(strings.TrimSpace(invoice))
	if inv != "" && typeID != "" {
		for _, e := range entries {
			if e.TypeID == typeID && strings.ToLower(strings.TrimSpace(e.Invoice)) == inv {
				return e, "same invoice number", true
			}
		}
	}
	return domain.LibraryEntry{}, "", false
}
