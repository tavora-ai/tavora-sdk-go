# @tavora/cli

The Tavora CLI distributed as an npm package. Wraps the Go binary built from
`tavora-cli/cmd/tavora/`. Standard pattern: `postinstall` downloads the
prebuilt binary for the user's platform from a GitHub Release, and a JS shim
in `bin/tavora.js` execs it with the user's argv.

## Install

```sh
npm i -g @tavora/cli
# or
pnpm add -g @tavora/cli
# or
yarn global add @tavora/cli
```

## What ships in the package

| File | Purpose |
|---|---|
| `package.json` | Declares the `tavora` bin entry + `postinstall` hook |
| `install.js` | Postinstall — picks the platform artifact, downloads + ungzips into `bin/` |
| `bin/tavora.js` | JS shim — npm's `bin` target. Execs the platform binary, forwards stdio + signals |
| `bin/tavora` (or `tavora.exe`) | The Go binary, populated by `install.js` at install time |

## Supported platforms

| Platform / arch | Release artifact |
|---|---|
| darwin / arm64 | `tavora-darwin-arm64.gz` |
| darwin / x64 | `tavora-darwin-amd64.gz` |
| linux / arm64 | `tavora-linux-arm64.gz` |
| linux / x64 | `tavora-linux-amd64.gz` |
| win32 / x64 | `tavora-windows-amd64.exe.gz` |

Unsupported platforms get a clear "no prebuilt binary" message during install.

## Skipping the download

For monorepo or offline installs where the postinstall network hop isn't
viable:

```sh
TAVORA_SKIP_DOWNLOAD=1 npm i -g @tavora/cli
```

The package installs without a binary; the user is expected to drop one at
`node_modules/@tavora/cli/bin/tavora` (or `tavora.exe`) manually.

## Releasing

Releases are automated by [`.github/workflows/release.yml`](../.github/workflows/release.yml).
On every `v*` tag push, the workflow:

1. Runs `goreleaser` to cut the GitHub Release with the tar.gz archives
   used by direct download and Homebrew.
2. Cross-compiles the five npm-shape `.gz` binaries
   (`tavora-<os>-<arch>.gz`, plus `tavora-windows-amd64.exe.gz`) and
   uploads them to the same Release.
3. Publishes `@tavora/cli` and then `create-tavora` to npm.

To cut a release:

```sh
# 1. Bump version in BOTH npm/package.json AND create-tavora/package.json.
#    Both must equal the tag — the workflow's first step verifies this.
# 2. Commit the bumps.
# 3. Tag and push.
git tag -a v0.0.1 -m "v0.0.1"
git push origin v0.0.1
```

Required repo secret: `NPM_TOKEN` (npm access token with publish rights
for both packages). `GITHUB_TOKEN` is auto-provided.

The order is load-bearing: the `.gz` upload happens *before*
`npm publish` so the postinstall download URL is reachable the moment
the package appears on the registry.

### Manual fallback

If you ever need to publish by hand (workflow broken, urgent fix), the
manual flow is: cross-compile + gzip with the same names listed in
`install.js`'s `ARTIFACTS` map, `gh release upload <tag> *.gz`, then
`npm publish --access public` from `npm/` and `create-tavora/`.

## Migration path: per-platform optional dependencies

The current single-package + postinstall-download approach is the simplest
to ship. Once install reliability or offline support becomes important,
migrate to the esbuild / prisma pattern:

- Publish one platform-specific package per target (`@tavora/cli-darwin-arm64`,
  `@tavora/cli-linux-x64`, …) — each containing only the binary.
- Use `optionalDependencies` + `os` / `cpu` in the main package so npm
  resolves and installs only the matching one.
- Drop `install.js` and the network hop entirely.

The bin shim (`bin/tavora.js`) stays — it just resolves the platform
package via `require.resolve` instead of `path.join(__dirname, …)`.
