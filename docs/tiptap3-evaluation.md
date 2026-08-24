# TipTap 3 evaluation

Status: **not approved for upgrade**.

The current editor remains on TipTap 2.27.2 with `tiptap-markdown` 0.8.10.
The Playwright golden suite in `tests/e2e/tiptap-roundtrip.spec.ts` exercises
rich text, tables, task lists, and fenced code blocks through the real Console
editor. The suite requires zero normalized Markdown diff and is the gate for
any future version comparison.

The current baseline includes one observed normalization rule: adjacent task
items are emitted with a blank line between them. This is recorded in the
golden fixture rather than hidden in a broad whitespace normalizer.

The repository deliberately does not perform a TipTap 3 upgrade in this
batch. The roadmap's permanent exclusions reject an active TipTap 3 upgrade,
and the upstream Markdown extension/version must be evaluated in an isolated
branch before changing the host editor. Table conversion regressions tracked
as #5750/#7435 remain mandatory cases. Until a TipTap 3 candidate is available
locally and passes the same golden corpus with zero diff, the current TipTap 2
stack stays the release candidate.
