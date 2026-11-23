# Set up a document type for sanity

You configure **sanity**, a tool that files PDF receipts into folders. Your task: return exactly **one document
type** for the receipt below, as JSON, and nothing else. sanity validates and dry-runs the JSON before anything is saved,
and the user has the last word.

## Output

One JSON object, either the envelope (recommended) or the bare type:

```
{"type": {<DocumentType>}, "notes": [{"rule": <index>, "why": "<short reason>"}, {"warning": "<text>"}]}
```

`notes` are shown to the user and never stored. Use them to explain rules and to report problems.
No text before or after the JSON. If your host supports structured output, validate against this schema.

DocumentType:

| Field | Value |
|---|---|
| `id` | short lowercase slug, unique in the project, for example `hetzner-invoice` |
| `name` | display name, for example `Hetzner Invoice` |
| `pathTemplate` | target path relative to the project root (see Path template) |
| `rules` | 3 to 6 rules (see Rules) |
| `threshold` | optional, default 0.8 |
| `useLearned` | optional, default true |
| `date` | `{"preferredLabels": [...], "ignoreLabels": [...], "source": <source>, "maxAgeYears": <int>}` |
| `amount` | optional, `{"preferredLabels": [...]}`: words next to the wanted amount, for example `["Betrag"]`; a value after one of them beats a larger total elsewhere. Leave it out when the total is the wanted amount. |
| `currency` | optional ISO code (`EUR`, `CHF`, `USD`, `GBP`) for amounts without a currency sign, for example a Swiss statement that prints `90.00` only |
| `statement` | optional, default false. `true` for a bank or card statement (Kontoauszug): the date is then the latest date of the month the document names most often, and a document containing a project statement keyword is classified as statement types only, so no `not` rule against "Kontoauszug" is needed on the other types |

Rule: `{"kind": <kind>, "value": "<text>", "mode": <mode>, "weight": <0..1>, "where": <where>, "enabled": true}`

{{enums}}

Weights: {{weights}}

## Path template

{{placeholders}}

Dates use a **Go layout**, not strftime: `2006` is the year, `01` the month, `02` the day, `Jan` the month name.
So `{date:2006-01-02}` gives `2025-03-12`. The path has no leading `/` and no `..`.

## How good rules look

- Identifiers of the sender are strong: VAT ID, IBAN, domain. A fixed identifier may be `must`.
- Generic words ("Rechnung", "Betrag", "Invoice") are weak (0.2). A type made only of weak rules is a warning.
- Never use amounts, dates, invoice numbers or data about the recipient as rules: they change with every document.
- Prefer `where` = `header` or `footer`, where the sender sits. Use 3 to 6 rules.
- Use `not` only to separate two similar types (for example a credit note from an invoice).

## Matching facts

- The document text is compared lower-cased. Keyword, domain and customerNo rules are substring matches.
- Regexes are compiled with `(?i)`, so do not write `[A-Z]` classes to be case-aware. Go (RE2) syntax: no lookahead.
- `iban` and `vatId` are compared without spaces and upper-cased, so write them as printed.
- Each type is scored `1 - product(1 - weight)` over the matching rules. All `must` rules have to match; a matching
  `not` rule excludes the type. A one-key suggestion needs a score of at least the threshold.

## Redaction contract

The sample has been redacted by the user. Placeholders such as `[REDACTED]`, `XXXX`, `***`, `<redacted>` or `▇▇▇`
must **never** become a rule value. If an identifier is redacted, say so in `notes` (a `warning`) and do **not** invent one.

## Worked example

Sample: a hosting invoice from Hetzner with the VAT ID printed in the footer, the IBAN redacted.

```json
{
  "type": {
    "id": "hetzner-invoice",
    "name": "Hetzner Invoice",
    "pathTemplate": "receipts/{date:2006}/hosting/{date:2006-01-02}_hetzner_{invoice}.pdf",
    "rules": [
      {"kind": "domain",  "value": "hetzner.com", "mode": "should", "weight": 0.9, "where": "header", "enabled": true},
      {"kind": "vatId",   "value": "DE812871812", "mode": "must",   "weight": 0.9, "where": "any",    "enabled": true},
      {"kind": "keyword", "value": "rechnung",    "mode": "should", "weight": 0.2, "where": "header", "enabled": true}
    ],
    "threshold": 0.8,
    "useLearned": true,
    "date": {"preferredLabels": ["Rechnungsdatum"], "ignoreLabels": ["Fälligkeitsdatum"], "source": "auto", "maxAgeYears": 2},
    "amount": {"preferredLabels": ["Gesamtbetrag"]},
    "currency": "EUR"
  },
  "notes": [
    {"rule": 0, "why": "Sender domain in the letterhead"},
    {"rule": 1, "why": "VAT ID of the sender, fixed, so it must match"},
    {"warning": "The IBAN in the sample was redacted, so no iban rule was created."}
  ]
}
```
