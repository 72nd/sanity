# testdata

Private scan samples for the recognition tests and OCR tuning. The PDFs, `src/`, `expected.json` and `measurements/`
are git-ignored; only this file is tracked.

- `*.pdf`: the samples (printed and scanned again on purpose, or text PDFs).
- `src/`: the clean originals of some samples. `ocrdump -ref` measures OCR word recall against them.
- `expected.json`: hand-checked date, invoice number, amount and currency per sample, plus optional type settings
  (`statement`, `typeCurrency`, `amountLabels`). `analysis/corpus_expected_test.go` reports the hit rate.
- `measurements/`: output of `ocrdump` and corpus runs (`sweep-*`, `text-*`, `expected-run*`). Regenerate, do not trust.

Useful runs (outside the agent sandbox, Vision needs it):

    go run ./cmd/ocrdump -file testdata/kontist.pdf -ref testdata/src/kontist_2025-03-01.pdf -sweep
    go test -count=1 -run TestCorpusExpected -v ./analysis
