# eval-ci — moved to tavora CLI

This example was promoted to a built-in subcommand of the `tavora` CLI on
2026-05-04. The standalone Go program is no longer maintained — the CLI
ships as a single binary, has the same functionality (poll, print results
table, exit non-zero on failure), and doesn't require a Go toolchain in
your CI runner.

## Use it

Install the [`tavora`](https://github.com/tavora-ai/tavora-tools) CLI, then:

```sh
# One-time setup: create a couple of sample cases (idempotent).
tavora evals seed

# Trigger a run, wait for completion, gate the CI build on results.
tavora evals run --gate --timeout 10m
```

`--gate` implies `--wait`; the command polls until the run completes,
prints a per-case PASS/FAIL table, and exits non-zero if any case fails.

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
          ./tavora evals run --gate --timeout 10m
        env:
          TAVORA_URL: ${{ secrets.TAVORA_URL }}
          TAVORA_API_KEY: ${{ secrets.TAVORA_API_KEY }}
```

## What this example used to do

A standalone program that called `client.RunEval()`, polled the run,
printed a results table, and exited non-zero on failure. About 200 lines
of Go. The CLI now does the same in `tavora evals run --gate` with no
maintenance overhead for users.

For the source as it was, see commit history before this README replaced
the program.
