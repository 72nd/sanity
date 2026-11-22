package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/72nd/sanity/domain"
)

// Ids of the sample document types in NewFake.
const (
	typeTelekom  = "telekom-invoice"
	typeVodafone = "vodafone-invoice"
	typeApple    = "apple-store"
	typeKontist  = "kontist-statement"
	typeCursor   = "cursor"
)

// Fake is an in-memory Store with sample data for the UI and for tests.
// Nothing is persisted.
type Fake struct {
	mu sync.Mutex
	m  memory
}

var _ Store = (*Fake)(nil)

// NewFake returns a Fake with the sample project "Muster GmbH".
func NewFake() *Fake {
	return &Fake{m: memory{p: sampleProject()}}
}

// Project implements Store.
func (f *Fake) Project(ctx context.Context) (domain.Project, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.m.snapshot(), nil
}

// Library implements Store.
func (f *Fake) Library(ctx context.Context) ([]domain.LibraryEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return cloneLibrary(f.m.p.Library), nil
}

// Types implements Store.
func (f *Fake) Types(ctx context.Context) ([]domain.DocumentType, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return cloneTypes(f.m.p.Types), nil
}

// SaveType implements Store.
func (f *Fake) SaveType(ctx context.Context, t domain.DocumentType) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m.saveType(t)
	return nil
}

// DeleteType implements Store.
func (f *Fake) DeleteType(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.m.deleteType(id)
}

// AddEntry implements Store.
func (f *Fake) AddEntry(ctx context.Context, e domain.LibraryEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.m.addEntry(e)
}

// RemoveEntry implements Store.
func (f *Fake) RemoveEntry(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.m.removeEntry(id)
}

func sampleProject() domain.Project {
	return domain.Project{
		Name:  "Muster GmbH",
		Root:  "/Users/demo/muster-gmbh",
		Inbox: domain.DefaultInbox,
		IgnoreTokens: []string{
			"DE123456789",                   // own VAT ID
			"Musterstraße 12, 10115 Berlin", // own address line
			"DE89370400440532013000",        // own IBAN (example)
		},
		Types:   sampleTypes(),
		Library: sampleLibrary(),
	}
}

func sampleTypes() []domain.DocumentType {
	return []domain.DocumentType{
		{
			ID:           typeTelekom,
			Name:         "Telekom Invoice",
			PathTemplate: "receipts/{date:2006}/telecom/{date:2006-01-02}_telekom_{invoice}.pdf",
			Rules: []domain.Rule{
				{Kind: "keyword", Value: "Telekom Deutschland GmbH", Mode: "should", Weight: 0.9, Where: "any", Enabled: true},
				{Kind: "keyword", Value: "Rechnungsnummer", Mode: "should", Weight: 0.5, Where: "header", Enabled: true},
				{Kind: "regex", Value: `Kundennummer:?\s*\d{8,12}`, Mode: "should", Weight: 0.5, Where: "any", Enabled: true},
				{Kind: "domain", Value: "telekom.de", Mode: "should", Weight: 0.2, Where: "footer", Enabled: true},
				{Kind: "filename", Value: "telekom", Mode: "should", Weight: 0.5, Where: "filename", Enabled: true},
			},
			Threshold:  0.8,
			UseLearned: true,
			Date: domain.DateConfig{
				PreferredLabels: []string{"Rechnungsdatum", "Datum"},
				IgnoreLabels:    []string{"Fällig bis", "Leistungszeitraum"},
				Source:          "text",
				MaxAgeYears:     3,
			},
		},
		{
			ID:           typeVodafone,
			Name:         "Vodafone Invoice",
			PathTemplate: "receipts/{date:2006}/vodafone/{date:2006-01-02}_vodafone_{invoice}.pdf",
			Rules: []domain.Rule{
				{Kind: "keyword", Value: "Vodafone GmbH", Mode: "should", Weight: 0.9, Where: "any", Enabled: true},
				{Kind: "keyword", Value: "Kundennummer", Mode: "should", Weight: 0.2, Where: "any", Enabled: true},
				{Kind: "domain", Value: "vodafone.de", Mode: "should", Weight: 0.5, Where: "footer", Enabled: true},
				{Kind: "filename", Value: "vodafone", Mode: "should", Weight: 0.5, Where: "filename", Enabled: true},
			},
			Threshold:  0.8,
			UseLearned: true,
			Date: domain.DateConfig{
				PreferredLabels: []string{"Rechnungsdatum"},
				IgnoreLabels:    []string{"Zahlungsziel"},
				Source:          "auto",
				MaxAgeYears:     3,
			},
		},
		{
			ID:           typeApple,
			Name:         "Apple Store",
			PathTemplate: "receipts/{date:2006}/apple/{date:2006-01-02}_apple_{invoice}.pdf",
			Rules: []domain.Rule{
				{Kind: "keyword", Value: "Apple Distribution International", Mode: "should", Weight: 0.9, Where: "footer", Enabled: true},
				{Kind: "keyword", Value: "Apple Store", Mode: "should", Weight: 0.5, Where: "any", Enabled: true},
				{Kind: "keyword", Value: "Bestellnummer", Mode: "should", Weight: 0.5, Where: "any", Enabled: true},
				{Kind: "regex", Value: `W\d{9,11}`, Mode: "should", Weight: 0.5, Where: "header", Enabled: true},
				{Kind: "domain", Value: "apple.com", Mode: "should", Weight: 0.2, Where: "any", Enabled: true},
			},
			Threshold:  0.8,
			UseLearned: true,
			Date: domain.DateConfig{
				PreferredLabels: []string{"Bestelldatum", "Datum"},
				Source:          "text",
				MaxAgeYears:     3,
			},
		},
		{
			ID:           typeKontist,
			Name:         "Kontoauszug Kontist",
			PathTemplate: "finance/{date:2006}/bank/{date:2006-01-02}_kontist_kontoauszug.pdf",
			Rules: []domain.Rule{
				{Kind: "keyword", Value: "Kontist", Mode: "should", Weight: 0.9, Where: "any", Enabled: true},
				{Kind: "keyword", Value: "Kontoauszug", Mode: "should", Weight: 0.9, Where: "header", Enabled: true},
				{Kind: "keyword", Value: "Saldo", Mode: "should", Weight: 0.5, Where: "any", Enabled: true},
				{Kind: "keyword", Value: "Solaris Bank", Mode: "should", Weight: 0.5, Where: "footer", Enabled: true},
				{Kind: "keyword", Value: "Rechnungsnummer", Mode: "not", Weight: 0.9, Where: "any", Enabled: true},
				{Kind: "filename", Value: "kontoauszug", Mode: "should", Weight: 0.5, Where: "filename", Enabled: true},
			},
			Threshold:  0.8,
			UseLearned: true,
			Date: domain.DateConfig{
				PreferredLabels: []string{"Zeitraum bis", "Kontoauszug vom"},
				Source:          "text",
				MaxAgeYears:     2,
			},
		},
		{
			ID:           typeCursor,
			Name:         "Cursor",
			PathTemplate: "receipts/{date:2006}/software/{date:2006-01-02}_cursor_{invoice}.pdf",
			Rules: []domain.Rule{
				{Kind: "keyword", Value: "Anysphere", Mode: "should", Weight: 0.9, Where: "footer", Enabled: true},
				{Kind: "keyword", Value: "Cursor Pro", Mode: "should", Weight: 0.5, Where: "any", Enabled: true},
				{Kind: "keyword", Value: "Cursor", Mode: "should", Weight: 0.2, Where: "any", Enabled: true},
				{Kind: "keyword", Value: "Cursor Team", Mode: "should", Weight: 0.2, Where: "any", Enabled: false},
				{Kind: "domain", Value: "cursor.com", Mode: "should", Weight: 0.5, Where: "any", Enabled: true},
				{Kind: "filename", Value: "cursor", Mode: "should", Weight: 0.5, Where: "filename", Enabled: true},
			},
			Threshold:  0.8,
			UseLearned: false,
			Date: domain.DateConfig{
				PreferredLabels: []string{"Invoice date"},
				Source:          "auto",
				MaxAgeYears:     3,
			},
		},
	}
}

// sampleReceipt describes one filed document. The path follows the type's
// path template.
type sampleReceipt struct {
	typeID  string
	dir     string // relative to the project root
	slug    string // file name part after the date
	date    time.Time
	invoice string // empty for documents without a number
	amount  string
}

func sampleLibrary() []domain.LibraryEntry {
	day := func(m time.Month, d int) time.Time {
		return time.Date(2025, m, d, 0, 0, 0, 0, time.UTC)
	}
	const (
		telecomDir  = "receipts/2025/telecom"
		vodafoneDir = "receipts/2025/vodafone"
		appleDir    = "receipts/2025/apple"
		softwareDir = "receipts/2025/software"
		bankDir     = "finance/2025/bank"
	)
	receipts := []sampleReceipt{
		{typeTelekom, telecomDir, "telekom", day(time.January, 8), "4500123401", "49.95 EUR"},
		{typeTelekom, telecomDir, "telekom", day(time.February, 7), "4500123877", "49.95 EUR"},
		{typeTelekom, telecomDir, "telekom", day(time.March, 7), "4500124310", "49.95 EUR"},
		{typeTelekom, telecomDir, "telekom", day(time.June, 9), "4500126552", "54.95 EUR"},
		{typeTelekom, telecomDir, "telekom", day(time.September, 8), "4500128019", "54.95 EUR"},
		{typeTelekom, telecomDir, "telekom", day(time.December, 8), "4500130477", "54.95 EUR"},

		{typeVodafone, vodafoneDir, "vodafone", day(time.February, 14), "VF-2025-0218734", "39.99 EUR"},
		{typeVodafone, vodafoneDir, "vodafone", day(time.May, 14), "VF-2025-0234401", "44.99 EUR"},
		{typeVodafone, vodafoneDir, "vodafone", day(time.August, 14), "VF-2025-0249907", "44.99 EUR"},
		{typeVodafone, vodafoneDir, "vodafone", day(time.November, 14), "VF-2025-0266312", "44.99 EUR"},

		{typeApple, appleDir, "apple", day(time.March, 22), "W3214578901", "10.99 EUR"},
		{typeApple, appleDir, "apple", day(time.July, 19), "W3221908445", "1099.00 EUR"},
		{typeApple, appleDir, "apple", day(time.October, 4), "W3229118702", "2.99 EUR"},
		{typeApple, appleDir, "apple", day(time.November, 4), "W3230114872", "24.99 EUR"},

		{typeCursor, softwareDir, "cursor", day(time.January, 3), "7A1F3C2E-0001", "18.40 EUR"},
		{typeCursor, softwareDir, "cursor", day(time.February, 3), "7A1F3C2E-0002", "18.40 EUR"},
		{typeCursor, softwareDir, "cursor", day(time.March, 3), "7A1F3C2E-0003", "18.40 EUR"},
		{typeCursor, softwareDir, "cursor", day(time.May, 3), "7A1F3C2E-0005", "18.40 EUR"},
		{typeCursor, softwareDir, "cursor", day(time.August, 3), "7A1F3C2E-0008", "18.40 EUR"},
		{typeCursor, softwareDir, "cursor", day(time.October, 3), "7A1F3C2E-0010", "18.40 EUR"},

		{typeKontist, bankDir, "kontist_kontoauszug", day(time.January, 31), "", ""},
		{typeKontist, bankDir, "kontist_kontoauszug", day(time.February, 28), "", ""},
		{typeKontist, bankDir, "kontist_kontoauszug", day(time.March, 31), "", ""},
		{typeKontist, bankDir, "kontist_kontoauszug", day(time.April, 30), "", ""},
		{typeKontist, bankDir, "kontist_kontoauszug", day(time.May, 31), "", ""},
	}

	out := make([]domain.LibraryEntry, 0, len(receipts))
	for i, r := range receipts {
		name := r.date.Format("2006-01-02") + "_" + r.slug
		if r.invoice != "" {
			name += "_" + r.invoice
		}
		path := r.dir + "/" + name + ".pdf"
		out = append(out, domain.LibraryEntry{
			ID:           fmt.Sprintf("lib-%02d", i+1),
			TypeID:       r.typeID,
			Path:         path,
			OriginalPath: fmt.Sprintf("/Users/demo/Downloads/%s_%s.pdf", r.slug, r.date.Format("20060102")),
			Date:         r.date,
			Invoice:      r.invoice,
			Amount:       r.amount,
			SHA256:       sampleHash("file:" + path),
			TextHash:     sampleHash("text:" + path),
			FiledAt:      r.date.AddDate(0, 0, 2).Add(9*time.Hour + 14*time.Minute),
		})
	}
	return out
}

// sampleHash returns a deterministic hex digest for fake data.
func sampleHash(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])
}
