# llm-judge

Minimal demo of LLM-as-judge scoring: ask Gemini to grade an assistant's
answer against a known ground-truth value on a 0–10 rubric.

About 80 lines of Go. Doesn't use the Tavora SDK — the judge call goes
to Gemini directly, since judging happens *outside* the Tavora pipeline
(the SDK's role is to produce the answer; LLM-as-judge scores it).

```sh
export GEMINI_API_KEY=...
go run .                                            # canned demo
go run . --answer "the total is $2,657.71"          # grade your own answer
```

For full RAG eval with this scoring pattern, use the
[`tavora` CLI](https://github.com/tavora-ai/tavora-tools):

```sh
tavora rag-eval judge --gate
```

The CLI runs the full pipeline (upload → ask → judge → report) against
the [tavora-testdata](https://github.com/tavora-ai/tavora-testdata) corpus.
