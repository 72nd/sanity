package presenter

import (
	"strings"
	"testing"
	"time"

	"github.com/72nd/sanity/domain"
)

func TestDateGroupKeysSortChronologically(t *testing.T) {
	d := func(y int, m time.Month) time.Time { return time.Date(y, m, 15, 0, 0, 0, 0, time.UTC) }
	tests := []struct {
		id   string
		t    time.Time
		want string
	}{
		{GroupYear, d(2026, 3), "2026"},
		{GroupQuarter, d(2026, 3), "2026-Q1"},
		{GroupQuarter, d(2026, 4), "2026-Q2"},
		{GroupQuarter, d(2026, 12), "2026-Q4"},
		{GroupMonth, d(2026, 3), "2026-03"},
		{GroupMonth, d(2026, 11), "2026-11"},
		{GroupMonth, time.Time{}, ""},
		{"nope", d(2026, 1), ""},
	}
	for _, tt := range tests {
		if got := DateGroupKey(tt.id, tt.t); got != tt.want {
			t.Errorf("DateGroupKey(%s, %v) = %q, want %q", tt.id, tt.t, got, tt.want)
		}
	}
	if !(DateGroupKey(GroupMonth, d(2026, 2)) < DateGroupKey(GroupMonth, d(2026, 10))) {
		t.Error("month keys must sort as strings")
	}
}

func TestDateGroupLabel(t *testing.T) {
	p := NewLibrary(nil, stubT)
	for _, tt := range []struct{ id, key, want string }{
		{GroupYear, "2026", "2026"},
		{GroupQuarter, "2026-Q1", "Q1 2026"},
		{GroupMonth, "2026-03", "03.2026"},
		{GroupMonth, "", "library.group.nodate"},
		{GroupMonth, "garbage", "garbage"},
	} {
		if got := p.DateGroupLabel(tt.id, tt.key); got != tt.want {
			t.Errorf("DateGroupLabel(%s, %q) = %q, want %q", tt.id, tt.key, got, tt.want)
		}
	}
}

func TestAmountValueAndSum(t *testing.T) {
	if v, ok := AmountValue("1.234,56 €"); !ok || v != 1234.56 {
		t.Errorf("AmountValue = %v %v", v, ok)
	}
	if _, ok := AmountValue("n/a"); ok {
		t.Error("text without digits must not parse")
	}
	p := NewLibrary(nil, stubT)
	rows := []Row{
		{Entry: domain.LibraryEntry{Amount: "10.50 EUR"}},
		{Entry: domain.LibraryEntry{Amount: "2,00 EUR"}},
		{Entry: domain.LibraryEntry{Amount: "$5.00"}},
		{Entry: domain.LibraryEntry{Amount: ""}},
	}
	if got := p.AmountSum(rows); got != "12.50 EUR, 5.00 USD" {
		t.Errorf("AmountSum = %q", got)
	}
	if got := p.AmountSum(rows[3:]); got != "" {
		t.Errorf("AmountSum without amounts = %q", got)
	}
	if !p.AmountLess(rows[3], rows[0]) || !p.AmountLess(rows[1], rows[0]) || p.AmountLess(rows[0], rows[1]) {
		t.Error("AmountLess must order numerically, empty first")
	}
}

func TestConditionsLabel(t *testing.T) {
	p := NewLibrary(nil, stubT)
	if got := p.ConditionsLabel(0); got != "library.conditions" {
		t.Errorf("0 = %q", got)
	}
	if got := p.ConditionsLabel(2); !strings.HasPrefix(got, "library.conditions.n") || !strings.HasSuffix(got, "2") {
		t.Errorf("2 = %q", got)
	}
}

func TestNextHitSkipsGroupHeaders(t *testing.T) {
	p := NewLibrary(nil, stubT)
	rows := []Row{{}, {Entry: domain.LibraryEntry{ID: "a", Path: "x"}}, {}, {Entry: domain.LibraryEntry{ID: "b", Path: "y"}}}
	if got := p.NextHit(rows, 1, "", true); got != 3 {
		t.Errorf("next = %d, want 3", got)
	}
	if got := p.NextHit(rows, 1, "", false); got != 3 { // wraps past the header at 0
		t.Errorf("prev = %d, want 3", got)
	}
}

func TestLibraryFilterKeys(t *testing.T) {
	k := NewKeys(nil)
	for r, want := range map[rune]string{'f': "filter.col", 'F': "conditions"} {
		if cmd, ok := k.Feed(ContextLibrary, r); cmd != want || !ok {
			t.Errorf("%c = %q %v, want %q", r, cmd, ok, want)
		}
	}
}
