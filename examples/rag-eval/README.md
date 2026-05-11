# rag-eval — moved to tavora CLI

This example was promoted to a built-in subcommand of the `tavora` CLI on
2026-05-04 (alongside its siblings `rag-eval-formats` and `rag-eval-judge`).
The standalone Go program is no longer maintained.

## Use it

Install the [`tavora`](https://github.com/tavora-ai/tavora-tools) CLI:

```sh
# Format coverage: does each supported format ingest + index?
tavora rag-eval formats --gate

# Answer accuracy: does the RAG endpoint answer correctly per
# structured ground truth? (Requires GEMINI_API_KEY.)
tavora rag-eval judge --gate --pass-threshold 7
```

Both default to the [tavora-testdata](https://github.com/tavora-ai/tavora-testdata)
corpus expected as a sibling clone.

## What this example used to do

A standalone program that uploaded a small support-doc corpus and ran
keyword-based retrieval + chat tests against it. About 460 lines of Go.
The `formats` and `judge` modes in `tavora rag-eval` cover the same
ground (and more) using the canonical tavora-testdata corpus instead of
a hand-curated support-doc corpus that didn't generalize.

The keyword-based retrieval-relevance mode wasn't ported — its case set
was too tightly coupled to the support-doc corpus. If you want a
general retrieval-relevance check, use `tavora rag-eval formats` (which
verifies each format is searchable) or write app-specific cases
via `tavora evals create` + `tavora evals run --gate`.

For the source as it was, see commit history before this README replaced
the program.
