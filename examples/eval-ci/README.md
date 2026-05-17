# eval-ci — moved to tavora CLI

This example was promoted to a built-in subcommand of the `tavora` CLI on
2026-05-04. The standalone Go program is no longer maintained — the CLI
ships as a single binary, has the same functionality (poll, print results
table, exit non-zero on failure), and doesn't require a Go toolchain in
your CI runner.

## Use it

Install the [`tavora`](https://github.com/tavora-ai/tavora-tools) CLI, then:

```sh
# Author eval cases as JSON files under tavora/agents/<id>/evals/
# (see https://docs.tavora.ai/tutorials/skills/) and ship them with:
tavora deploy

# Trigger an advisory run against the agent's pinned suite, gate CI
# on the result:
tavora evals run <agent> --gate --timeout 10m
```

`--gate` implies `--wait`; the command polls until the run completes,
prints a per-case PASS/FAIL table, and exits non-zero if any case
fails the suite's pass threshold. Eval failures are advisory at the
platform level (no promotion gate as of the 2026-05-11 slim-down),
but `--gate` still lets you block CI on them.

## GitHub Actions

```yaml
# .github/workflows/eval.yml
jobs:
  eval-gate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: |
          curl -L https://github.com/tavora-ai/tavora-tools/releases/latest/download/tavora-linux-amd64 -o tavora
          chmod +x tavora
          ./tavora evals run support --gate --timeout 10m
        env:
          TAVORA_URL: ${{ secrets.TAVORA_URL }}
          TAVORA_API_KEY: ${{ secrets.TAVORA_API_KEY }}
```

## What this example used to do

A standalone program that called `client.RunEval()` (a cross-suite
ad-hoc run endpoint), polled the run, printed a results table, and
exited non-zero on failure. About 200 lines of Go.

`client.RunEval()` came off the SDK with the Phase-12 promotion-gate
teardown; runs are now per-agent against the agent's pinned suite
(`runAgentEval` on the SDK, `tavora evals run <agent>` on the CLI).

For the source as it was, see commit history before this README replaced
the program.
