"use strict";

const { spawnSync } = require("child_process");
const fs = require("fs");
const path = require("path");
const https = require("https");
const { pipeline } = require("stream/promises");
const { createWriteStream, createGunzip } = require("fs");
const { createUnzip } = require("zlib");
const os = require("os");

const REPO = "harshmendhe-arch/dhriti";
const VERSION = process.env.DHRITI_VERSION || "latest";

function platformTag() {
  const plat = process.platform;
  const arch = process.arch;
  if (plat === "win32") {
    if (arch !== "x64" && arch !== "arm64") throw new Error("Unsupported Windows arch: " + arch);
    return { os: "windows", arch: arch === "arm64" ? "arm64" : "x86_64", ext: "zip" };
  }
  if (plat === "darwin") {
    if (arch !== "x64" && arch !== "arm64") throw new Error("Unsupported macOS arch: " + arch);
    return { os: "mac", arch: arch === "arm64" ? "arm64" : "x86_64", ext: "tar.gz" };
  }
  if (plat === "linux") {
    if (arch !== "x64" && arch !== "arm64") throw new Error("Unsupported Linux arch: " + arch);
    return { os: "linux", arch: arch === "arm64" ? "arm64" : "x86_64", ext: "tar.gz" };
  }
  throw new Error("Unsupported platform: " + plat);
}

function releaseUrl() {
  const { os: o, arch, ext } = platformTag();
  const name = `dhriti-${o}-${arch}.${ext}`;
  if (VERSION === "latest") {
    return `https://github.com/${REPO}/releases/latest/download/${name}`;
  }
  const v = VERSION.startsWith("v") ? VERSION : "v" + VERSION;
  return `https://github.com/${REPO}/releases/download/${v}/${name}`;
}

function follow(url, redirects = 5) {
  return new Promise((resolve, reject) => {
    https
      .get(url, (res) => {
        if (
          res.statusCode >= 300 &&
          res.statusCode < 400 &&
          res.headers.location &&
          redirects > 0
        ) {
          res.resume();
          resolve(follow(res.headers.location, redirects - 1));
          return;
        }
        if (res.statusCode !== 200) {
          reject(new Error(`Download failed: HTTP ${res.statusCode} for ${url}`));
          res.resume();
          return;
        }
        resolve(res);
      })
      .on("error", reject);
  });
}

async function main() {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "dhriti-"));
  const { ext } = platformTag();
  const archive = path.join(tmp, "dhriti-archive." + ext);
  const url = releaseUrl();

  console.log(`Downloading dhriti from ${url}`);
  const res = await follow(url);
  await pipeline(res, createWriteStream(archive));

  const destBin = path.join(
    __dirname,
    process.platform === "win32" ? "dhriti.exe" : "dhriti"
  );

  if (ext === "tar.gz") {
    const r = spawnSync("tar", ["-xzf", archive, "-C", tmp], { stdio: "inherit" });
    if (r.status !== 0) throw new Error("tar extract failed");
  } else {
    const r = spawnSync(
      "powershell",
      ["-Command", `Expand-Archive -Path '${archive}' -DestinationPath '${tmp}' -Force`],
      { stdio: "inherit" }
    );
    if (r.status !== 0) throw new Error("unzip failed");
  }

  const extracted = path.join(tmp, process.platform === "win32" ? "dhriti.exe" : "dhriti");
  if (!fs.existsSync(extracted)) {
    // search
    const walk = (dir) => {
      for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
        const p = path.join(dir, e.name);
        if (e.isDirectory()) {
          const f = walk(p);
          if (f) return f;
        } else if (e.name === "dhriti" || e.name === "dhriti.exe") {
          return p;
        }
      }
      return null;
    };
    const found = walk(tmp);
    if (!found) throw new Error("binary not found in archive");
    fs.copyFileSync(found, destBin);
  } else {
    fs.copyFileSync(extracted, destBin);
  }

  if (process.platform !== "win32") {
    fs.chmodSync(destBin, 0o755);
  }

  fs.rmSync(tmp, { recursive: true, force: true });
  console.log("Installed dhriti binary to", destBin);
}

main().catch((err) => {
  console.error(err.message || err);
  process.exit(1);
});
