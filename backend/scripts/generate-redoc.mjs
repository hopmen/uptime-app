import { readFileSync, writeFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const backendDir = fileURLToPath(new URL("../", import.meta.url));
const result = spawnSync("npx", [
  "--yes", "@redocly/cli@2.65.0", "build-docs", "docs/openapi.json",
  "--output", "docs/redoc.html", "--disableGoogleFont",
], { cwd: backendDir, stdio: "inherit" });

if (result.error) throw result.error;
if (result.status !== 0) process.exit(result.status ?? 1);

// Normalize whitespace emitted by the upstream template for a clean Git diff.
const output = new URL("../docs/redoc.html", import.meta.url);
writeFileSync(output, readFileSync(output, "utf8").replace(/[\t ]+$/gm, ""));
