# @cloudbox/statuspulse (npm distribution)

Source layout for the npm distribution of the StatusPulse CLI.

Users install:

```sh
npx @cloudbox/statuspulse --help
npm i -g @cloudbox/statuspulse
```

## How it works (`optionalDependencies` pattern)

At release time we publish **N+1** packages, all at the same version:

| Package | Contents |
|---|---|
| `@cloudbox/statuspulse` | Meta package. Node shim + `optionalDependencies` listing every platform package. |
| `@cloudbox/statuspulse-linux-x64` | Linux x86_64 prebuilt binary. |
| `@cloudbox/statuspulse-linux-arm64` | Linux ARM64 prebuilt binary. |
| `@cloudbox/statuspulse-darwin-x64` | macOS Intel prebuilt binary. |
| `@cloudbox/statuspulse-darwin-arm64` | macOS Apple Silicon prebuilt binary. |
| `@cloudbox/statuspulse-win32-x64` | Windows x86_64 prebuilt binary. |

Each platform package declares `"os"` and `"cpu"` so npm/pnpm/yarn only downloads the one matching the host. The meta package's `bin/statuspulse.js` shim resolves the correct platform package at runtime and execs the binary.

Same industry pattern as esbuild, biome, turbo, swc, bun, @sentry/cli.

## Build flow (done by CI, not locally)

The `.github/workflows/release.yml` `npm` job runs after GoReleaser produces `dist/`. It invokes:

1. `npm/scripts/build-platform-packages.js` — extracts each GoReleaser tarball into `npm/dist/platforms/<name>/bin/` and writes a tailored `package.json`.
2. `npm publish` over each platform package.
3. `npm/scripts/build-meta-package.js` — copies `bin/statuspulse.js` + writes a meta `package.json` with `optionalDependencies` pinned to the same version.
4. `npm publish` the meta.

Nothing in this directory is published directly — only the generated contents of `npm/dist/` are.
