package domain

import "testing"

func TestOfficeRel(t *testing.T) {
	cases := map[string]string{
		"receipts/2026/x.pdf": "../receipts/2026/x.pdf",
		"office/a.pdf":        "a.pdf",
		"office/inbox/a.pdf":  "inbox/a.pdf",
		"./receipts/x.pdf":    "../receipts/x.pdf",
		`receipts\2026\x.pdf`: "../receipts/2026/x.pdf",
		`office\inbox\a.pdf`:  "inbox/a.pdf",
		"officeX/a.pdf":       "../officeX/a.pdf",
	}
	for in, want := range cases {
		if got := OfficeRel(in); got != want {
			t.Errorf("OfficeRel(%q) = %q, want %q", in, got, want)
		}
	}
}
