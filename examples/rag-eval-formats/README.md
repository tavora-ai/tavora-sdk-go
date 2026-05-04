# rag-eval-formats — moved to tavora CLI

This example was promoted to a built-in subcommand of the `tavora` CLI on
2026-05-04. The standalone Go program is no longer maintained.

## Use it

Install the [`tavora`](https://github.com/tavora-ai/tavora-tools) CLI and
clone the [`tavora-testdata`](https://github.com/tavora-ai/tavora-testdata)
corpus as a sibling. Then:

```sh
# Smoke-test ingestion + search across pdf, md, txt, csv, html.
tavora rag-eval formats --gate
```

`--gate` exits non-zero if any format has zero searchable documents,
which is the right CI signal for "the extractor regressed on this format."

## What this example used to do

Uploaded N samples per format from `tavora-testdata/extraction/kreuzberg/`,
waited for processing, ran a retrieval round-trip per file, and printed
a per-format coverage table (uploaded / processed / searchable). About
360 lines of Go, almost all of which became `tavora rag-eval formats`.

For the source as it was, see commit history before this README replaced
the program.
