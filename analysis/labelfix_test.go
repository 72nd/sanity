package analysis

import (
	"testing"
	"time"

	"github.com/72nd/sanity/domain"
)

func TestFixLabels(t *testing.T) {
	text := "Date: 25. January 2026 at 00:07\nINVOIGE DAIE\n24 Jan 2026\nSEQUENCE NO.\n1-12852631517"
	fixed := FixLabels(text)
	if fixed == text {
		t.Fatal("misread label not fixed")
	}
	c := FindDates(fixed, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), "en", domain.DateConfig{})
	if len(c) == 0 || c[0].Date.Format("2006-01-02") != "2026-01-24" {
		t.Errorf("top date %v, want 2026-01-24", c)
	}
	for _, same := range []string{"Invoice date", "Total amount", "Rechnung 2025", "Invoice\nSEQUENCE NO."} {
		if FixLabels(same) != same {
			t.Errorf("%q must stay", same)
		}
	}
}
