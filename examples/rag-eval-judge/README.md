# rag-eval-judge — moved to tavora CLI

This example was promoted to a built-in subcommand of the `tavora` CLI on
2026-05-04. The standalone Go program is no longer maintained.

## Use it

Install the [`tavora`](https://github.com/tavora-ai/tavora-tools) CLI,
clone the [`tavora-testdata`](https://github.com/tavora-ai/tavora-testdata)
corpus as a sibling, then:

```sh
export TAVORA_URL=...
export TAVORA_API_KEY=...
export GEMINI_API_KEY=...   # required by the LLM judge

tavora rag-eval judge --gate --pass-threshold 7
```

`--gate` exits non-zero if any judgment scores below `--pass-threshold`
(default 7) or errors out. Suitable as the final step of a CI eval job.

## What this example used to do

Uploaded the 10-invoice fixture from `tavora-testdata/extraction/invoices/`,
asked the RAG endpoint questions about each (vendor, total, date,
invoice_number, currency), and used Gemini to score each answer 0-10
against `cases.json` ground truth. About 440 lines of Go, almost all
of which became `tavora rag-eval judge`.

For the source as it was, see commit history before this README replaced
the program.
