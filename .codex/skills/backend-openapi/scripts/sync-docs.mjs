import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

// Resolve from the installed project skill, independently of the caller's cwd.
const backendDir = fileURLToPath(new URL("../../../../backend/", import.meta.url));
const contracts = ["docs/openapi.json", "docs/openapi.yaml"];

function run(command, args) {
  console.log(`> ${command} ${args.join(" ")}`);
  const result = spawnSync(command, args, { cwd: backendDir, stdio: "inherit" });
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(`${command} failed (${result.signal ?? result.status ?? "unknown status"})`);
  }
}

try {
  run("go", ["generate", "./docs"]);
  const snapshots = contracts.map((path) => readFileSync(join(backendDir, path)));
  run("go", ["generate", "./docs"]);
  for (const [index, path] of contracts.entries()) {
    const current = readFileSync(join(backendDir, path));
    if (!snapshots[index].equals(current)) {
      throw new Error(`OpenAPI generation is not reproducible: ${path}`);
    }
  }
  run(process.execPath, ["scripts/generate-redoc.mjs"]);
  console.log("OpenAPI synchronized; generation is reproducible; ReDoc generated.");
} catch (error) {
  console.error(`Documentation sync failed: ${error.message}`);
  process.exitCode = 1;
}
