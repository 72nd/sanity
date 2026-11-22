---
name: sanity-type-setup
description: Use when the user wants a new sanity document type (receipt or invoice type with recognition rules and a path template) drafted from a redacted sample PDF. Produces a JSON snippet and checks it with `sanity types check` until it has no errors.
---

# Draft a sanity document type from a sample

sanity files PDF receipts. A document type is JSON: id, name, path template, recognition rules, date settings.
You only configure; sanity never calls an LLM while sorting.

## Steps

1. **Get the prompt.** The rules, enum values and a worked example are in `prompts/type-setup.md` (the source of truth;
   `{{...}}` markers are filled from the Go code at runtime; the values are `RuleKinds`, `RuleModes`, `RuleWheres` and
   `DateSources` in `analysis/typecheck.go`). Follow it.
2. **Read the sample.** Ask the user for a *redacted* sample (own IBAN, address, amounts removed). For a scan, get the
   text from OCR first. Never invent an identifier that is redacted; say so in `notes` instead. Placeholders such as
   `[REDACTED]` or `XXXX` must not become a rule value.
3. **Write the snippet** to a file, for example `snippet.json`: the envelope `{"type": {...}, "notes": [...]}` or a bare
   type. Use 3 to 6 rules: sender identifiers (domain, VAT ID, IBAN) are strong, generic words weak (0.2), no amounts,
   dates or invoice numbers. Weights may be numbers or `weak`/`normal`/`strong` (0.2/0.5/0.9).
4. **Check it** from the sanity project folder (macOS only, it reads the PDF):

   ```
   go run . types check snippet.json sample.pdf [more.pdf ...] [--project <root>]
   ```

   It prints a JSON report: `Findings` (level `error`, `warning`, `ok`, a stable `Code`, `RuleIdx`), `Samples` (score,
   `Confident`, other types above 0), `Library` (filed documents of other types that the new rules also match) and
   `Skipped`. Exit code 0 means no errors, 1 errors, 2 usage.
5. **Refine** until there are no errors: fix the rule named by `RuleIdx` and `Code` (for example `rule.regex`,
   `rule.redacted`, `template.unsafe`, `id.collision` where `Argument` is a free id). Warnings do not block, but read them:
   `sample.weak` (add a stronger identifier), `library.falsePositive` (a rule is too generic), `rule.ignoreToken` (the
   value is one of the project's own identifiers, drop it).
6. **Hand over** the final JSON. The user pastes it in Types, Import type, reviews and saves. Do not edit
   `office/sanity.json` yourself.

## Facts

- Text is compared lower-cased; regexes use `(?i)` and Go (RE2) syntax; IBAN and VAT ID are compared without spaces.
- Path template placeholders: `{date:<Go layout>}` (`2006-01-02`), `{type}`, `{invoice}`, `{amount}`, `{original}`,
  `{text:<label>}`. No leading `/`, no `..`.
