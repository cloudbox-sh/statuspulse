#!/usr/bin/env node
// Meta-package shim: resolves the platform-specific @cloudbox/statuspulse-*
// package at runtime and execs the binary it ships. Matches the
// optionalDependencies pattern used by esbuild, biome, bun, swc, etc.

const { spawnSync } = require("node:child_process");
const { existsSync } = require("node:fs");
const path = require("node:path");

const { platform, arch } = process;

const PLATFORM_MAP = {
  "linux-x64": { pkg: "@cloudbox/statuspulse-linux-x64", bin: "statuspulse" },
  "linux-arm64": { pkg: "@cloudbox/statuspulse-linux-arm64", bin: "statuspulse" },
  "darwin-x64": { pkg: "@cloudbox/statuspulse-darwin-x64", bin: "statuspulse" },
  "darwin-arm64": { pkg: "@cloudbox/statuspulse-darwin-arm64", bin: "statuspulse" },
  "win32-x64": { pkg: "@cloudbox/statuspulse-win32-x64", bin: "statuspulse.exe" },
};

const key = `${platform}-${arch}`;
const target = PLATFORM_MAP[key];

if (!target) {
  console.error(
    `statuspulse: unsupported platform ${key}.\n` +
      `Supported: ${Object.keys(PLATFORM_MAP).join(", ")}.\n` +
      `Install manually: https://github.com/cloudbox-sh/statuspulse/releases`
  );
  process.exit(1);
}

let pkgDir;
try {
  pkgDir = path.dirname(require.resolve(`${target.pkg}/package.json`));
} catch {
  console.error(
    `statuspulse: platform package ${target.pkg} is not installed.\n` +
      `This usually means your package manager stripped optionalDependencies.\n` +
      `Reinstall without --no-optional, or install the platform package directly:\n` +
      `  npm i ${target.pkg}`
  );
  process.exit(1);
}

const binPath = path.join(pkgDir, "bin", target.bin);
if (!existsSync(binPath)) {
  console.error(`statuspulse: binary missing at ${binPath}. Reinstall @cloudbox/statuspulse.`);
  process.exit(1);
}

const result = spawnSync(binPath, process.argv.slice(2), {
  stdio: "inherit",
  windowsHide: true,
});

if (result.error) {
  console.error(`statuspulse: failed to execute binary: ${result.error.message}`);
  process.exit(1);
}

process.exit(result.status ?? 1);
