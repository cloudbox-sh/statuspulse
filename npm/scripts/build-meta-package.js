#!/usr/bin/env node
// Builds the meta @cloudbox/statuspulse npm package.
//
// Expects:
//   VERSION env var — version number without leading "v"
// Produces:
//   npm/dist/meta/           — meta package directory ready to `npm publish`
//   ├── package.json
//   ├── README.md
//   └── bin/statuspulse.js
//
// Run: node npm/scripts/build-meta-package.js

"use strict";

const fs = require("node:fs");
const path = require("node:path");

const VERSION = process.env.VERSION;
if (!VERSION) {
  console.error("build-meta-package: VERSION env var is required (no leading 'v').");
  process.exit(1);
}

const REPO_ROOT = path.resolve(__dirname, "..", "..");
const NPM_DIR = path.join(REPO_ROOT, "npm");
const OUT_DIR = path.join(NPM_DIR, "dist", "meta");

const PLATFORM_PACKAGES = [
  "@cloudbox/statuspulse-linux-x64",
  "@cloudbox/statuspulse-linux-arm64",
  "@cloudbox/statuspulse-darwin-x64",
  "@cloudbox/statuspulse-darwin-arm64",
  "@cloudbox/statuspulse-win32-x64",
];

const optionalDependencies = Object.fromEntries(
  PLATFORM_PACKAGES.map((p) => [p, VERSION])
);

fs.rmSync(OUT_DIR, { recursive: true, force: true });
fs.mkdirSync(path.join(OUT_DIR, "bin"), { recursive: true });

const pkgJson = {
  name: "@cloudbox/statuspulse",
  version: VERSION,
  description: "StatusPulse CLI — hosted status pages from the terminal (also an MCP server).",
  keywords: [
    "statuspulse",
    "cloudbox",
    "status-page",
    "uptime",
    "monitoring",
    "incident",
    "cli",
    "mcp",
  ],
  homepage: "https://statuspulse.cloudbox.sh",
  bugs: "https://github.com/cloudbox-sh/statuspulse/issues",
  repository: {
    type: "git",
    url: "git+https://github.com/cloudbox-sh/statuspulse.git",
  },
  license: "MIT",
  bin: {
    statuspulse: "bin/statuspulse.js",
  },
  files: ["bin/"],
  engines: {
    node: ">=18",
  },
  optionalDependencies,
};

fs.writeFileSync(
  path.join(OUT_DIR, "package.json"),
  JSON.stringify(pkgJson, null, 2) + "\n"
);

fs.copyFileSync(
  path.join(NPM_DIR, "bin", "statuspulse.js"),
  path.join(OUT_DIR, "bin", "statuspulse.js")
);

// Ship a user-facing README instead of the internal build notes.
const rootReadme = path.join(REPO_ROOT, "README.md");
fs.copyFileSync(rootReadme, path.join(OUT_DIR, "README.md"));

console.log(`built @cloudbox/statuspulse @ ${VERSION} in ${OUT_DIR}`);
