#!/usr/bin/env node
"use strict";

const { spawnSync } = require("child_process");
const path = require("path");
const fs = require("fs");

const bin = path.join(
  __dirname,
  process.platform === "win32" ? "dhriti.exe" : "dhriti"
);

if (!fs.existsSync(bin)) {
  console.error(
    "dhriti binary not found. Run: node install.js (or reinstall the package)."
  );
  process.exit(1);
}

const result = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
process.exit(result.status === null ? 1 : result.status);
